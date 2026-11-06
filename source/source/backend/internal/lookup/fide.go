package lookup

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Player is the normalized result of any lookup source.
type Player struct {
	Source      string `json:"source"` // fide | ffe
	LastName    string `json:"lastName"`
	FirstName   string `json:"firstName"`
	Title       string `json:"title"`
	Fed         string `json:"fed"`
	Sex         string `json:"sex"`
	BirthDate   string `json:"birthDate"`
	Category    string `json:"category"`
	Club        string `json:"club"`
	FideID      string `json:"fideId"`
	FFEID       string `json:"ffeId"`
	Licence     string `json:"licence"`
	Standard    int    `json:"standard"`
	StandardTyp string `json:"standardType"`
	Rapid       int    `json:"rapid"`
	RapidTyp    string `json:"rapidType"`
	Blitz       int    `json:"blitz"`
	BlitzTyp    string `json:"blitzType"`
}

// ErrNotFound is returned when a player does not exist in a source.
var ErrNotFound = errors.New("joueur introuvable")

// FIDEClient fetches player cards from ratings.fide.com.
type FIDEClient struct {
	HTTP    *http.Client
	BaseURL string
}

var fideIDRe = regexp.MustCompile(`^\d{3,10}$`)

// Get fetches and parses https://ratings.fide.com/profile/{id}.
func (c *FIDEClient) Get(ctx context.Context, id string) (*Player, error) {
	id = strings.TrimSpace(id)
	if !fideIDRe.MatchString(id) {
		return nil, fmt.Errorf("identifiant FIDE invalide %q", id)
	}
	base := c.BaseURL
	if base == "" {
		base = "https://ratings.fide.com"
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/profile/"+id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Roque chess pairing)")
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FIDE injoignable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FIDE: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	p, err := ParseFIDEProfile(string(body))
	if err != nil {
		return nil, err
	}
	p.FideID = id
	return p, nil
}

var (
	reTitleH1   = regexp.MustCompile(`(?s)class="player-title[^"]*"[^>]*>(.*?)</h1>`)
	reOldTitle  = regexp.MustCompile(`(?s)class="profile-top-title[^"]*"[^>]*>(.*?)</div>`)
	reCountry   = regexp.MustCompile(`(?s)class="profile-info-country[^"]*"[^>]*>(.*?)</div>`)
	reFlag      = regexp.MustCompile(`/flags/([a-z]{2}(?:-[a-z]+)?)\.svg`)
	reByear     = regexp.MustCompile(`(?s)class="profile-info-byear[^"]*"[^>]*>\s*(\d{4})`)
	reSex       = regexp.MustCompile(`(?s)class="profile-info-sex[^"]*"[^>]*>\s*([A-Za-z]+)`)
	reInfoTitle = regexp.MustCompile(`(?s)class="profile-info-title[^"]*"[^>]*>\s*<p>(.*?)</p>`)
	reTags      = regexp.MustCompile(`<[^>]+>`)
	reFirstNum  = regexp.MustCompile(`<p>\s*(\d{3,4})\s*</p>`)
)

// ParseFIDEProfile extracts a player from the FIDE profile HTML.
func ParseFIDEProfile(page string) (*Player, error) {
	name := firstGroup(page, reTitleH1)
	if name == "" {
		name = firstGroup(page, reOldTitle)
	}
	name = clean(name)
	if name == "" {
		return nil, ErrNotFound
	}
	p := &Player{Source: "fide"}
	p.LastName, p.FirstName = splitName(name)

	if blk := firstGroup(page, reCountry); blk != "" {
		if m := reFlag.FindStringSubmatch(blk); m != nil {
			p.Fed = iso2ToFIDE[m[1]]
		}
		if p.Fed == "" {
			p.Fed = countryToFIDE[strings.ToLower(clean(blk))]
		}
	}
	p.BirthDate = firstGroup(page, reByear)
	switch strings.ToLower(firstGroup(page, reSex)) {
	case "male":
		p.Sex = "m"
	case "female":
		p.Sex = "f"
	}
	p.Title = titleFromLabel(clean(firstGroup(page, reInfoTitle)))
	p.Standard = blockRating(page, "profile-standart")
	p.Rapid = blockRating(page, "profile-rapid")
	p.Blitz = blockRating(page, "profile-blitz")
	for _, typ := range []*string{&p.StandardTyp, &p.RapidTyp, &p.BlitzTyp} {
		*typ = "F"
	}
	return p, nil
}

// blockRating reads the first number of a rating block, bounded so that a
// missing value cannot be filled by the next block.
func blockRating(page, class string) int {
	i := strings.Index(page, class+" profile-game")
	if i < 0 {
		i = strings.Index(page, class)
	}
	if i < 0 {
		return 0
	}
	end := i + 400
	if end > len(page) {
		end = len(page)
	}
	if m := reFirstNum.FindStringSubmatch(page[i:end]); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func firstGroup(s string, re *regexp.Regexp) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func clean(s string) string {
	s = reTags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

// splitName handles "Carlsen, Magnus" and falls back to "First Last".
func splitName(name string) (last, first string) {
	if i := strings.Index(name, ","); i >= 0 {
		return strings.TrimSpace(name[:i]), strings.TrimSpace(name[i+1:])
	}
	parts := strings.Fields(name)
	if len(parts) < 2 {
		return name, ""
	}
	return parts[len(parts)-1], strings.Join(parts[:len(parts)-1], " ")
}

func titleFromLabel(label string) string {
	switch strings.ToLower(label) {
	case "grandmaster", "gm":
		return "GM"
	case "international master", "im":
		return "IM"
	case "woman grandmaster", "wgm":
		return "WGM"
	case "fide master", "fm":
		return "FM"
	case "woman international master", "wim":
		return "WIM"
	case "candidate master", "cm":
		return "CM"
	case "woman fide master", "wfm":
		return "WFM"
	case "woman candidate master", "wcm":
		return "WCM"
	}
	return ""
}

// iso2ToFIDE maps the flag file names used on ratings.fide.com (ISO 3166-1
// alpha-2) to FIDE federation codes.
var iso2ToFIDE = map[string]string{
	"fr": "FRA", "ch": "SUI", "be": "BEL", "lu": "LUX", "mc": "MNC", "de": "GER", "it": "ITA",
	"es": "ESP", "pt": "POR", "gb-eng": "ENG", "gb-sct": "SCO", "gb-wls": "WLS", "ie": "IRL",
	"nl": "NED", "at": "AUT", "pl": "POL", "cz": "CZE", "sk": "SVK", "hu": "HUN", "ro": "ROU",
	"bg": "BUL", "gr": "GRE", "tr": "TUR", "ru": "RUS", "ua": "UKR", "by": "BLR", "lt": "LTU",
	"lv": "LAT", "ee": "EST", "fi": "FIN", "se": "SWE", "no": "NOR", "dk": "DEN", "is": "ISL",
	"hr": "CRO", "si": "SLO", "rs": "SRB", "ba": "BIH", "me": "MNE", "mk": "MKD", "al": "ALB",
	"md": "MDA", "ge": "GEO", "am": "ARM", "az": "AZE", "kz": "KAZ", "uz": "UZB", "il": "ISR",
	"us": "USA", "ca": "CAN", "mx": "MEX", "br": "BRA", "ar": "ARG", "cl": "CHI", "co": "COL",
	"pe": "PER", "ve": "VEN", "cu": "CUB", "uy": "URU", "in": "IND", "cn": "CHN", "jp": "JPN",
	"kr": "KOR", "vn": "VIE", "ph": "PHI", "id": "INA", "my": "MAS", "sg": "SGP", "au": "AUS",
	"nz": "NZL", "ir": "IRI", "eg": "EGY", "ma": "MAR", "dz": "ALG", "tn": "TUN", "za": "RSA",
	"ng": "NGR", "sn": "SEN", "ci": "CIV", "cm": "CMR", "mg": "MAD", "ad": "AND", "mt": "MLT",
	"cy": "CYP", "li": "LIE", "sm": "SMR", "fo": "FAI", "mn": "MGL", "bd": "BAN", "lk": "SRI",
	"ae": "UAE", "qa": "QAT", "sa": "KSA", "lb": "LBN", "jo": "JOR", "sy": "SYR", "iq": "IRQ",
}

var countryToFIDE = map[string]string{
	"france": "FRA", "switzerland": "SUI", "belgium": "BEL", "luxembourg": "LUX", "monaco": "MNC",
	"germany": "GER", "italy": "ITA", "spain": "ESP", "portugal": "POR", "england": "ENG",
	"scotland": "SCO", "wales": "WLS", "ireland": "IRL", "netherlands": "NED", "austria": "AUT",
	"poland": "POL", "czech republic": "CZE", "czechia": "CZE", "hungary": "HUN", "romania": "ROU",
	"norway": "NOR", "sweden": "SWE", "denmark": "DEN", "finland": "FIN", "russia": "RUS",
	"ukraine": "UKR", "united states of america": "USA", "usa": "USA", "india": "IND",
	"china": "CHN", "canada": "CAN", "israel": "ISR", "armenia": "ARM", "georgia": "GEO",
	"azerbaijan": "AZE", "fide": "FID",
}
