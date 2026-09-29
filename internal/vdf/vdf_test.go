package vdf

import "testing"

const appManifest = `"AppState"
{
	"appid"		"2072450"
	"universe"		"1"
	"name"		"Like a Dragon: Infinite Wealth"
	"StateFlags"		"4"
	"InstalledDepots"
	{
		"2072451"
		{
			"manifest"		"123"
		}
	}
}
`

const libraryFolders = `"libraryfolders"
{
	// comment line
	"0"
	{
		"path"		"/home/u/.local/share/Steam"
		"apps"
		{
			"228980"		"452851932"
		}
	}
	"1"
	{
		"path"		"/mnt/games/SteamLibrary"
	}
}
`

func TestAppManifest(t *testing.T) {
	n, err := Parse(appManifest)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := n.Get("AppState", "name"); got != "Like a Dragon: Infinite Wealth" {
		t.Errorf("name = %q", got)
	}
	if got, _ := n.Get("appstate", "APPID"); got != "2072450" {
		t.Errorf("appid = %q (keys should be case-insensitive)", got)
	}
	if got, _ := n.Get("AppState", "InstalledDepots", "2072451", "manifest"); got != "123" {
		t.Errorf("nested = %q", got)
	}
	if _, ok := n.Get("AppState", "InstalledDepots"); ok {
		t.Error("Get on a block should report !ok")
	}
	if _, ok := n.Get("AppState", "missing"); ok {
		t.Error("missing key should report !ok")
	}
}

func TestLibraryFolders(t *testing.T) {
	n, err := Parse(libraryFolders)
	if err != nil {
		t.Fatal(err)
	}
	lf, ok := n.Child("libraryfolders")
	if !ok {
		t.Fatal("no libraryfolders block")
	}
	want := map[string]string{"0": "/home/u/.local/share/Steam", "1": "/mnt/games/SteamLibrary"}
	for k, w := range want {
		if got, _ := lf.Get(k, "path"); got != w {
			t.Errorf("%s path = %q, want %q", k, got, w)
		}
	}
}

func TestEscapesAndBare(t *testing.T) {
	n, err := Parse(`root { "a" "say \"hi\" \\ ok" b c }`)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := n.Get("root", "a"); got != `say "hi" \ ok` {
		t.Errorf("a = %q", got)
	}
	if got, _ := n.Get("root", "b"); got != "c" {
		t.Errorf("b = %q", got)
	}
}

func TestErrors(t *testing.T) {
	for _, in := range []string{
		`"a" { "b" "c"`,
		`"a" "unterminated`,
		`}`,
		`"a" { "b" }`,
		`{`,
	} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}
