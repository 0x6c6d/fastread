package input

// epub_drm.go detects encrypted (DRM) EPUBs from META-INF/encryption.xml and rejects them. It
// only reads the list of encrypted entries; nothing is ever decrypted or de-obfuscated.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const epubEncryptionPath = "META-INF/encryption.xml"

// errDRM is returned (wrapped) for EPUBs whose content documents are encrypted.
var errDRM = fmt.Errorf("%w: encrypted EPUB (DRM) not supported", ErrUnsupported)

// checkEncryption parses META-INF/encryption.xml (if present) and fails when any of the
// given spine entry names is encrypted.
func checkEncryption(z *zipArchive, spine []string, lim Limits) error {
	if !z.has(epubEncryptionPath) {
		return nil
	}
	b, err := z.read(epubEncryptionPath)
	if err != nil {
		return err
	}
	encrypted := map[string]bool{}
	err = walkXML(newXMLDecoder(bytes.NewReader(b)), lim.MaxDepth, func(tok xml.Token, _ int) error {
		t, ok := tok.(xml.StartElement)
		if !ok || t.Name.Local != "CipherReference" {
			return nil
		}
		name, err := encryptionEntryName(xmlAttr(t, "URI"))
		if err != nil {
			return err
		}
		encrypted[name] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrCorruptEPUB, epubEncryptionPath, err)
	}
	for _, name := range spine {
		if encrypted[name] {
			return errDRM
		}
	}
	return nil
}

// encryptionEntryName turns a CipherReference URI (relative to the zip root) into a zip entry
// name: the fragment is dropped, the URI unescaped, a leading "/" removed and the path cleaned.
func encryptionEntryName(uri string) (string, error) {
	if i := strings.IndexByte(uri, '#'); i >= 0 {
		uri = uri[:i]
	}
	u, err := url.PathUnescape(uri)
	if err != nil {
		return "", fmt.Errorf("CipherReference URI %q: %w", uri, err)
	}
	return path.Clean(strings.TrimPrefix(u, "/")), nil
}
