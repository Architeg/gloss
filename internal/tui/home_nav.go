package tui

// Dummy / placeholder URLs for docs and support (replace with real links when ready).
const (
	URLReadmeDocs     = "https://github.com/Architeg/gloss#readme"
	URLGitHubSponsors = "https://github.com/sponsors/Architeg"
	URLKoFi           = "https://ko-fi.com/architeg"
)

// SupportLink is one entry in the home support row.
type SupportLink struct {
	Icon  string
	Label string
	URL   string
}

// HomeSupportLinks is the secondary support row below the main menu.
var HomeSupportLinks = []SupportLink{
	{"›","GitHub Sponsors", URLGitHubSponsors},
	{"›","Ko-fi", URLKoFi},,
}

// homeSection tracks focus on the home screen.
type homeSection int

const (
	homeSectionMenu homeSection = iota
	homeSectionSupport
)
