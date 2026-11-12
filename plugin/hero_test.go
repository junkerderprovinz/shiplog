package plugin

import (
	"regexp"
	"strings"
	"testing"
)

// TestAPhoneKeepsTheWholeWidthForTheName keeps the banner's name readable on
// a phone. The room the wider layouts keep for the silhouette would leave it
// two letters there, so the text runs over the silhouette, which sits behind it.
func TestAPhoneKeepsTheWholeWidthForTheName(t *testing.T) {
	css := read(t, "styles/settings.css")
	narrow := strings.Index(css, "@media (max-width: 1023px)")
	phone := strings.Index(css, "@media (max-width: 599px)")
	if narrow < 0 || phone < narrow {
		t.Fatal("settings.css has no phone rule after the narrow one")
	}
	if !regexp.MustCompile(`^@media \(max-width: 599px\) \{\s*\.sl-hero-text \{ max-width: none; \}`).MatchString(css[phone:]) {
		t.Errorf("the phone rule does not free the name: %.80s", css[phone:])
	}
}
