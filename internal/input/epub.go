package input

// epub.go loads EPUB books from memory. Zip entry names are only ever map keys of the
// zipArchive: they are resolved with package path (forward slashes, no filesystem) and nothing
// is extracted or written anywhere.

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const epubContainerPath = "META-INF/container.xml"

// loadEPUB opens the zip in b, reads the linear XHTML spine items in spine order and returns
// their paragraphs concatenated. An encrypted (DRM) spine is rejected by checkEncryption before
// any spine item is read (ErrUnsupported); every other error wraps ErrCorruptEPUB.
func loadEPUB(b []byte, lim Limits) ([]string, error) {
	var paras []string
	err := recoverAs(ErrCorruptEPUB, func() error {
		z, err := openZip(b, lim)
		if err != nil {
			return err
		}
		names, err := epubSpine(z, lim)
		if err != nil {
			return err
		}
		if err := checkEncryption(z, names, lim); err != nil {
			return err
		}
		for _, name := range names {
			doc, err := z.read(name)
			if err != nil {
				return err
			}
			paras = append(paras, xhtmlParagraphs(doc)...)
		}
		return nil
	})
	if errors.Is(err, errDRM) {
		return nil, err
	}
	if err != nil {
		return nil, corruptEPUB(err)
	}
	return paras, nil
}

// corruptEPUB makes sure err matches ErrCorruptEPUB while keeping its chain.
func corruptEPUB(err error) error {
	if errors.Is(err, ErrCorruptEPUB) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrCorruptEPUB, err)
}

type epubItem struct {
	href, mediaType string
}

// epubSpine returns the zip entry names of the linear spine items in spine order.
func epubSpine(z *zipArchive, lim Limits) ([]string, error) {
	opfPath, err := epubRootfile(z, lim)
	if err != nil {
		return nil, err
	}
	if !z.has(opfPath) {
		return nil, fmt.Errorf("%w: package document %q not in the archive", ErrCorruptEPUB, opfPath)
	}
	opf, err := z.read(opfPath)
	if err != nil {
		return nil, err
	}

	items := map[string]epubItem{}
	var (
		idrefs              []string
		inManifest, inSpine bool
	)
	err = walkXML(newXMLDecoder(bytes.NewReader(opf)), lim.MaxDepth, func(tok xml.Token, _ int) error {
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "manifest":
				inManifest = true
			case "spine":
				inSpine = true
			case "item":
				if !inManifest {
					return nil
				}
				id := xmlAttr(t, "id")
				if _, dup := items[id]; id != "" && !dup {
					items[id] = epubItem{href: xmlAttr(t, "href"), mediaType: xmlAttr(t, "media-type")}
				}
			case "itemref":
				if !inSpine {
					return nil
				}
				idref := xmlAttr(t, "idref")
				if _, ok := items[idref]; !ok {
					return fmt.Errorf("spine idref %q not in the manifest", idref)
				}
				if strings.TrimSpace(xmlAttr(t, "linear")) != "no" {
					idrefs = append(idrefs, idref)
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "manifest":
				inManifest = false
			case "spine":
				inSpine = false
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: package document %q: %w", ErrCorruptEPUB, opfPath, err)
	}

	var names []string
	for _, idref := range idrefs {
		it := items[idref]
		switch strings.ToLower(strings.TrimSpace(it.mediaType)) {
		case "application/xhtml+xml", "text/html":
		default:
			continue
		}
		name, err := epubResolve(z, opfPath, it.href)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: empty spine (no linear XHTML item)", ErrCorruptEPUB)
	}
	return names, nil
}

// epubRootfile returns the full-path attribute of the first rootfile in META-INF/container.xml.
func epubRootfile(z *zipArchive, lim Limits) (string, error) {
	if !z.has(epubContainerPath) {
		return "", fmt.Errorf("%w: no %s", ErrCorruptEPUB, epubContainerPath)
	}
	b, err := z.read(epubContainerPath)
	if err != nil {
		return "", err
	}
	errFound := errors.New("found")
	var (
		full  string
		found bool
	)
	err = walkXML(newXMLDecoder(bytes.NewReader(b)), lim.MaxDepth, func(tok xml.Token, _ int) error {
		if t, ok := tok.(xml.StartElement); ok && t.Name.Local == "rootfile" {
			full, found = xmlAttr(t, "full-path"), true
			return errFound
		}
		return nil
	})
	if err != nil && !errors.Is(err, errFound) {
		return "", fmt.Errorf("%w: %s: %w", ErrCorruptEPUB, epubContainerPath, err)
	}
	if !found {
		return "", fmt.Errorf("%w: %s has no rootfile", ErrCorruptEPUB, epubContainerPath)
	}
	if full == "" {
		return "", fmt.Errorf("%w: %s rootfile has no full-path", ErrCorruptEPUB, epubContainerPath)
	}
	return full, nil
}

// epubResolve turns a manifest href (relative to the OPF) into a zip entry name. Results that
// escape the archive root or are not in the archive are rejected.
func epubResolve(z *zipArchive, opfPath, href string) (string, error) {
	if i := strings.IndexByte(href, '#'); i >= 0 {
		href = href[:i]
	}
	h, err := url.PathUnescape(href)
	if err != nil {
		return "", fmt.Errorf("%w: href %q: %w", ErrCorruptEPUB, href, err)
	}
	if h == "" || strings.HasPrefix(h, "/") {
		return "", fmt.Errorf("%w: href %q is not a relative entry name", ErrCorruptEPUB, href)
	}
	name := path.Clean(path.Join(path.Dir(opfPath), h))
	if strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("%w: href %q escapes the archive", ErrCorruptEPUB, href)
	}
	if !z.has(name) {
		return "", fmt.Errorf("%w: href %q: entry %q not in the archive", ErrCorruptEPUB, href, name)
	}
	return name, nil
}

// xmlAttr returns the value of the first attribute with this local name (namespace ignored).
func xmlAttr(e xml.StartElement, local string) string {
	for _, a := range e.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
