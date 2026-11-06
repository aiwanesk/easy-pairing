package chess

import (
	"fmt"
	"sort"
	"strings"
)

// PairingDiagnosis explains why no Swiss pairing exists for the next round.
type PairingDiagnosis struct {
	Summary     string   `json:"summary"`
	Reasons     []string `json:"reasons"`
	Suggestions []string `json:"suggestions"`
}

// Lines flattens the diagnosis for an error payload.
func (d PairingDiagnosis) Lines() []string {
	out := append([]string{}, d.Reasons...)
	for _, s := range d.Suggestions {
		out = append(out, "→ "+s)
	}
	return out
}

// colourState returns the absolute colour constraint of a player before
// round next (FIDE C.04.1 / C.04.3): 'w' or 'b' if he must get that colour
// (colour difference beyond ±1 or the same colour in his last two games),
// 0 otherwise.
func (t *Tournament) colourState(id, next int) byte {
	var seq []byte
	for i := 0; i < next-1 && i < len(t.Rounds); i++ {
		if o, ok := t.OutcomeIn(t.Rounds[i], id); ok && o.Played {
			seq = append(seq, o.Color)
		}
	}
	diff := 0
	for _, c := range seq {
		if c == 'w' {
			diff++
		} else {
			diff--
		}
	}
	switch {
	case diff > 1:
		return 'b'
	case diff < -1:
		return 'w'
	}
	if k := len(seq); k >= 2 && seq[k-1] == seq[k-2] {
		if seq[k-1] == 'w' {
			return 'b'
		}
		return 'w'
	}
	return 0
}

// DiagnoseSwissFailure looks for the reason bbpPairings found no pairing
// for the next round: the absolute criteria (no rematch, forbidden pairs,
// absolute colours, at most one point without playing for the exempt) leave
// some players without a possible opponent, or the compatibility graph has
// no perfect matching.
func (t *Tournament) DiagnoseSwissFailure() PairingDiagnosis {
	next := len(t.Rounds) + 1
	played := len(t.Rounds)
	var ids []int
	for _, p := range t.Participants() {
		if t.AbsenceFor(p, next) == "" {
			ids = append(ids, p.ID)
		}
	}
	sort.Ints(ids)
	forb := t.forbidden()
	scores := t.Scores(played)
	lastRound := next == t.TotalRounds
	maxScore := float64(played) * t.PointsWin
	topscorer := func(id int) bool { return lastRound && scores[id] > maxScore/2 }
	colour := map[int]byte{}
	for _, id := range ids {
		colour[id] = t.colourState(id, next)
	}
	why := func(a, b int) string {
		if t.Met(a, b, played) {
			return "déjà rencontré"
		}
		x, y := a, b
		if x > y {
			x, y = y, x
		}
		if forb[[2]int{x, y}] {
			return "appariement interdit"
		}
		if colour[a] != 0 && colour[a] == colour[b] && !(topscorer(a) || topscorer(b)) {
			return "doivent tous deux avoir les " + map[byte]string{'w': "Blancs", 'b': "Noirs"}[colour[a]]
		}
		return ""
	}
	n := len(ids)
	idx := map[int]int{}
	for i, id := range ids {
		idx[id] = i
	}
	adj := make([][]int, n+1) // vertex n = exempt (PAB) when the count is odd
	degree := make([]int, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if why(ids[i], ids[j]) == "" {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
				degree[i]++
				degree[j]++
			}
		}
	}
	odd := n%2 == 1
	var pabOK []int
	if odd {
		for i, id := range ids {
			if !t.hadPointWithoutPlaying(id, played) {
				pabOK = append(pabOK, id)
				adj[i] = append(adj[i], n)
				adj[n] = append(adj[n], i)
			}
		}
	}
	size := n
	if odd {
		size = n + 1
	}
	matched := maxMatching(adj[:size])

	d := PairingDiagnosis{}
	need := size / 2
	if matched >= need {
		d.Summary = "Les règles absolues (pas de revanche, appariements interdits, couleurs, exempt) laissent un appariement possible : le blocage vient des autres critères du moteur. Contrôlez l'appariement à la main ou contactez le support avec l'export TRF."
		d.Suggestions = append(d.Suggestions, "Exportez le TRF (onglet Joueurs ou Rondes) et vérifiez-le dans un autre logiciel (bbpPairings, JaVaFo).")
		return d
	}
	d.Summary = fmt.Sprintf("Impossible d'apparier la ronde %d : avec les %d joueurs présents, on ne peut former que %d échiquier(s) sur les %d nécessaires sans enfreindre une règle absolue.",
		next, n, matched, need)

	// Players with no possible opponent.
	for i, id := range ids {
		if degree[i] > 0 {
			continue
		}
		causes := map[string][]string{}
		for _, other := range ids {
			if other != id {
				if c := why(id, other); c != "" {
					causes[c] = append(causes[c], t.name(other))
				}
			}
		}
		var parts []string
		for _, c := range []string{"déjà rencontré", "appariement interdit"} {
			if len(causes[c]) > 0 {
				parts = append(parts, fmt.Sprintf("%s : %s", c, strings.Join(causes[c], ", ")))
			}
		}
		for c, who := range causes {
			if strings.HasPrefix(c, "doivent") {
				parts = append(parts, fmt.Sprintf("couleur (%s %s) : %s", t.name(id), strings.TrimPrefix(c, "doivent tous deux "), strings.Join(who, ", ")))
			}
		}
		line := fmt.Sprintf("%s n'a aucun adversaire possible — %s.", t.name(id), strings.Join(parts, " ; "))
		if odd && t.hadPointWithoutPlaying(id, played) {
			line += " Il a déjà eu un point sans jouer : il ne peut pas non plus être exempt."
		}
		d.Reasons = append(d.Reasons, line)
	}
	if odd && len(pabOK) == 0 {
		d.Reasons = append(d.Reasons, fmt.Sprintf("Nombre impair de joueurs (%d) et tous ont déjà reçu un point sans jouer (exempt, forfait, bye 1 point) : personne ne peut être exempt.", n))
	}
	if len(d.Reasons) == 0 {
		// Groups that cannot be closed: list the most constrained players.
		type pc struct {
			id, deg int
		}
		var tight []pc
		for i, id := range ids {
			if degree[i] <= 2 {
				tight = append(tight, pc{id, degree[i]})
			}
		}
		sort.Slice(tight, func(a, b int) bool { return tight[a].deg < tight[b].deg })
		for _, x := range tight {
			var opp []string
			for _, j := range adj[idx[x.id]] {
				if j < n {
					opp = append(opp, t.name(ids[j]))
				}
			}
			d.Reasons = append(d.Reasons, fmt.Sprintf("%s ne peut jouer que contre %s.", t.name(x.id), strings.Join(opp, " ou ")))
		}
		d.Reasons = append(d.Reasons, "Ces joueurs se disputent les mêmes adversaires : une fois ceux-ci appariés, il reste des joueurs qui se sont tous déjà rencontrés.")
	}
	d.Suggestions = append(d.Suggestions,
		"Le tournoi a peut-être plus de rondes que ce que le nombre de joueurs permet : réduisez le nombre de rondes (Réglages) ou ajoutez des joueurs.",
		"Accordez un bye ½ ou une absence pour cette ronde à un des joueurs cités (Rondes → présents et exempts).")
	if len(t.ForbiddenGroups) > 0 {
		d.Suggestions = append(d.Suggestions, "Assouplissez les appariements interdits (Joueurs → appariements interdits).")
	}
	if !t.FIDEMode {
		d.Suggestions = append(d.Suggestions, "Appariez la ronde à la main : l'outil signalera les revanches et les problèmes de couleur sans les bloquer.")
	}
	return d
}

// maxMatching returns the size of a maximum matching of a general graph
// (Edmonds' blossom algorithm, O(V³)).
func maxMatching(adj [][]int) int {
	n := len(adj)
	match := make([]int, n)
	p := make([]int, n)
	base := make([]int, n)
	used := make([]bool, n)
	blossom := make([]bool, n)
	q := make([]int, 0, n)
	for i := range match {
		match[i] = -1
	}
	lca := func(a, b int) int {
		seen := make([]bool, n)
		for {
			a = base[a]
			seen[a] = true
			if match[a] == -1 {
				break
			}
			a = p[match[a]]
		}
		for {
			b = base[b]
			if seen[b] {
				return b
			}
			b = p[match[b]]
		}
	}
	markPath := func(v, b, children int) {
		for base[v] != b {
			blossom[base[v]], blossom[base[match[v]]] = true, true
			p[v] = children
			children = match[v]
			v = p[match[v]]
		}
	}
	findPath := func(root int) int {
		for i := range used {
			used[i], p[i], base[i] = false, -1, i
		}
		used[root] = true
		q = q[:0]
		q = append(q, root)
		for h := 0; h < len(q); h++ {
			v := q[h]
			for _, to := range adj[v] {
				if base[v] == base[to] || match[v] == to {
					continue
				}
				if to == root || match[to] != -1 && p[match[to]] != -1 {
					cur := lca(v, to)
					for i := range blossom {
						blossom[i] = false
					}
					markPath(v, cur, to)
					markPath(to, cur, v)
					for i := 0; i < n; i++ {
						if blossom[base[i]] {
							base[i] = cur
							if !used[i] {
								used[i] = true
								q = append(q, i)
							}
						}
					}
				} else if p[to] == -1 {
					p[to] = v
					if match[to] == -1 {
						return to
					}
					used[match[to]] = true
					q = append(q, match[to])
				}
			}
		}
		return -1
	}
	size := 0
	for i := 0; i < n; i++ {
		if match[i] != -1 {
			continue
		}
		v := findPath(i)
		if v == -1 {
			continue
		}
		size++
		for v != -1 {
			pv := p[v]
			ppv := match[pv]
			match[v], match[pv] = pv, v
			v = ppv
		}
	}
	return size
}
