package art

import (
	"os"
	"testing"
)

func TestFindOnlineReal(t *testing.T) {
	if os.Getenv("WVFRM_NET") == "" {
		t.Skip("set WVFRM_NET=1 to hit the network")
	}
	data, src, err := FindOnline("Daft Punk", "Discovery")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(data); err != nil {
		t.Fatalf("undecodable image from %s: %v", src, err)
	}
	t.Logf("found via %s (%d bytes)", src, len(data))
	if _, _, err := FindOnline("Nobody Realistic", "An Album That Does Not Exist 48213"); err == nil {
		t.Fatal("expected no match for a made-up album")
	}
}

func TestSimplify(t *testing.T) {
	if simplify("Random Access Memories (Deluxe)") != "random access memories deluxe" {
		t.Fatal(simplify("Random Access Memories (Deluxe)"))
	}
}
