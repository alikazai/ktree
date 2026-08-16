package install

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeManagedArtifactReturnsBodyForTamperDetection(t *testing.T) {
	bundle := CoreSkillBundle()
	artifact, err := EncodeManagedArtifact(bundle, "claude")
	if err != nil {
		t.Fatalf("EncodeManagedArtifact() error = %v", err)
	}

	tamperedArtifact := strings.Replace(artifact, "canonical installer baseline.", "tampered installer baseline.", 1)

	decoded, err := DecodeManagedArtifact(tamperedArtifact)
	if err != nil {
		t.Fatalf("DecodeManagedArtifact() error = %v", err)
	}
	if decoded == nil || decoded.Metadata == nil {
		t.Fatal("DecodeManagedArtifact() metadata missing, want managed artifact")
	}
	if hashContent(decoded.Content) == decoded.Metadata.Hash {
		t.Fatalf("hashContent(decoded.Content) = %q, want mismatch from metadata hash %q", hashContent(decoded.Content), decoded.Metadata.Hash)
	}
}

func TestDecodeManagedArtifactReturnsUnmanagedPlainMarkdown(t *testing.T) {
	decoded, err := DecodeManagedArtifact("# plain skill\n")
	if err != nil {
		t.Fatalf("DecodeManagedArtifact() error = %v", err)
	}
	if decoded == nil {
		t.Fatal("DecodeManagedArtifact() = nil, want unmanaged artifact")
	}
	if decoded.Metadata != nil {
		t.Fatalf("DecodeManagedArtifact().Metadata = %+v, want nil", decoded.Metadata)
	}
	if decoded.Content != "# plain skill\n" {
		t.Fatalf("DecodeManagedArtifact().Content = %q, want %q", decoded.Content, "# plain skill\n")
	}
}

func TestDecodeManagedArtifactErrorsOnMalformedMetadataJSON(t *testing.T) {
	_, err := DecodeManagedArtifact("<!-- ktree:metadata {bad json} -->\n\n# ktree core skill\n")
	if !errors.Is(err, ErrManagedMetadataMalformed) {
		t.Fatalf("DecodeManagedArtifact() error = %v, want %v", err, ErrManagedMetadataMalformed)
	}
}

func TestDecodeManagedArtifactErrorsOnMetadataMissingRequiredFields(t *testing.T) {
	_, err := DecodeManagedArtifact("<!-- ktree:metadata {\"ID\":\"skill-core\"} -->\n\n# ktree core skill\n")
	if !errors.Is(err, ErrManagedMetadataMalformed) {
		t.Fatalf("DecodeManagedArtifact() error = %v, want %v", err, ErrManagedMetadataMalformed)
	}
	if err == nil || !strings.Contains(err.Error(), "missing required") {
		t.Fatalf("DecodeManagedArtifact() error = %v, want missing required fields detail", err)
	}
}

func TestDecodeManagedArtifactErrorsOnMissingTerminator(t *testing.T) {
	_, err := DecodeManagedArtifact("<!-- ktree:metadata {\"ID\":\"skill-core\"}\n# ktree core skill\n")
	if !errors.Is(err, ErrManagedMetadataMalformed) {
		t.Fatalf("DecodeManagedArtifact() error = %v, want %v", err, ErrManagedMetadataMalformed)
	}
}

func TestDecodeManagedArtifactToleratesCRLF(t *testing.T) {
	decoded, err := DecodeManagedArtifact("<!-- ktree:metadata {\"ID\":\"skill-core\",\"Version\":\"1.0.0\",\"Hash\":\"abc\",\"Target\":\"claude\",\"InstalledAtUnix\":0} -->\r\n\r\n# ktree core skill\r\n")
	if err != nil {
		t.Fatalf("DecodeManagedArtifact() error = %v", err)
	}
	if decoded == nil || decoded.Metadata == nil {
		t.Fatal("DecodeManagedArtifact() = nil metadata, want managed artifact")
	}
	if decoded.Content != "# ktree core skill\n" {
		t.Fatalf("DecodeManagedArtifact().Content = %q, want %q", decoded.Content, "# ktree core skill\n")
	}
}
