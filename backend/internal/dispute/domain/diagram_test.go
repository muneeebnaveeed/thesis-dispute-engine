package domain

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/platform/golden"
)

// The thesis figure of the lifecycle (make thesis:figures renders it); a change to the transition table fails here
// until the golden file, and with it the figure, is regenerated.
func TestStateMachineDiagram(t *testing.T) {
	golden.Check(t, "state-machine.dot", []byte(diagram(t)))
}

type phase struct {
	id, name  string
	fill, ink string
	corner    string // where the title sits, clear of the arrows: two of t, b, l, r
	states    []State
}

// Presentation only: the phases group and colour states for the reader; the transitions all come from the table.
func phases() []phase {
	return []phase{
		{"investigation", "Investigation", "#e8f1fb", "#3f73a8", "tl",
			[]State{StateInvestigating, StateQuestionnaireSent, StateQuestionnaireReceived}},
		{"money", "Money to the Customer", "#e7f5ea", "#3b8a50", "tr",
			[]State{StateProvisionalCreditIssued, StateFastRefundIssued, StateSEPANQARefundIssued}},
		{"chargeback", "Chargeback", "#fdefe1", "#b8691f", "tr",
			[]State{StateChargebackFiled, StateChargebackAcknowledged, StateEvidenceSubmitted, StateChargebackLost, StateChargebackWon}},
		{"settlement", "Settlement", "#f2eaf8", "#7a4ea3", "br",
			[]State{StateProvisionalCreditReversed, StateFinalCreditIssued}},
	}
}

// Presentation only: states drawn in one straight column.
func columns() map[State]string {
	return map[State]string{
		StateChargebackLost: "lost", StateProvisionalCreditReversed: "lost",
		StateChargebackWon: "won", StateFinalCreditIssued: "won",
	}
}

// from is a state, or a phase id when the step leaves more than one state of that phase; such a step is drawn
// from a junction at the phase's last state.
type arrow struct {
	from string
	to   State
}

type set map[string]bool

func (s set) String() string {
	var out []string
	for _, r := range []string{"SEPA", "PSD2", "Reg E", "Reg Z"} {
		if s[r] {
			out = append(out, r)
		}
	}
	return strings.Join(out, ", ")
}

func diagram(t *testing.T) string {
	t.Helper()
	short := map[Regime]string{
		RegimeEUSEPADirectDebit: "SEPA", RegimeEUPSD2Card: "PSD2", RegimeUSRegE: "Reg E", RegimeUSRegZ: "Reg Z",
	}
	phaseOf := map[State]phase{}
	for _, p := range phases() {
		for _, s := range p.states {
			phaseOf[s] = p
		}
	}

	// Each event is named by the state it enters, so a step is (from, to) plus the regimes that allow it.
	steps := map[arrow]set{}
	sources := map[arrow]map[State]bool{}
	reach := map[State]set{}
	for _, regime := range AllRegimes() {
		r := short[regime]
		for _, s := range reachable(t, regime) {
			if reach[s] == nil {
				reach[s] = set{}
			}
			reach[s][r] = true
			for _, e := range AllEvents() {
				if e == EventAppeal {
					continue
				}
				n, err := (Dispute{Regime: regime, State: s}).Apply(e)
				if err != nil {
					continue
				}
				k := arrow{string(s), n.State}
				if p, ok := phaseOf[s]; ok && phaseOf[n.State].id != p.id {
					k.from = p.id
				}
				if steps[k] == nil {
					steps[k], sources[k] = set{}, map[State]bool{}
				}
				steps[k][r] = true
				sources[k][s] = true
			}
		}
	}
	arrows := map[arrow]set{}
	junction, onward := map[string]bool{}, map[string]bool{}
	for k, rs := range steps {
		if src := sources[k]; len(src) == 1 {
			for s := range src {
				k.from = string(s)
			}
		} else {
			junction[k.from] = true
			if k.to != StateClosed {
				onward[k.from] = true
			}
		}
		arrows[k] = rs
	}

	var b strings.Builder
	b.WriteString("digraph lifecycle {\n")
	b.WriteString("  rankdir=TB; splines=ortho; nodesep=0.45; ranksep=0.5; newrank=true;\n")
	b.WriteString("  node [shape=box, style=\"rounded,filled\", fillcolor=white, color=\"gray35\", fontname=\"Times\", fontsize=15];\n")
	b.WriteString("  edge [color=\"gray35\", fontname=\"Times\", fontsize=14, penwidth=1.8, arrowsize=1.2];\n")
	// A state's regimes are shown on its box, or on its phase when the phase's states all share them.
	node := func(s State, shared bool) {
		l := label(string(s))
		if rs := reach[s].String(); !shared && len(reach[s]) < len(AllRegimes()) && !strings.Contains(l, rs) {
			l += " (" + rs + ")"
		}
		attrs := ""
		if g, ok := columns()[s]; ok {
			attrs = ", group=" + g
		}
		fmt.Fprintf(&b, "  %s [label=%q%s];\n", s, l, attrs)
	}
	node(StateInitiated, false)
	for _, p := range phases() {
		shared := true
		for _, s := range p.states {
			shared = shared && reach[s].String() == reach[p.states[0]].String()
		}
		title := p.name
		if shared && len(reach[p.states[0]]) < len(AllRegimes()) {
			title += "\n(" + reach[p.states[0]].String() + ")"
		}
		fmt.Fprintf(&b, "  subgraph cluster_%s {\n    label=%q; labelloc=%c; labeljust=%c; fontname=\"Times\"; fontsize=13;\n",
			p.id, title, p.corner[0], p.corner[1])
		fmt.Fprintf(&b, "    style=solid; penwidth=1.4; bgcolor=%q; pencolor=%q; fontcolor=%q;\n", p.fill, p.ink, p.ink)
		for _, s := range p.states {
			b.WriteString("  ")
			node(s, shared)
		}
		if junction[p.id] {
			// A junction whose steps all end the dispute sits beside the phase; one leading on sits below it.
			last := p.states[len(p.states)-1]
			fmt.Fprintf(&b, "    %s_exit [shape=point, width=0.09, color=%q, fillcolor=%q];\n", p.id, p.ink, p.ink)
			if onward[p.id] {
				fmt.Fprintf(&b, "    %s -> %s_exit [style=invis, weight=10];\n", last, p.id)
			} else {
				fmt.Fprintf(&b, "    { rank=same; %s; %s_exit; }\n", last, p.id)
			}
		}
		b.WriteString("  }\n")
	}

	rank := map[string]int{}
	for i, s := range AllStates() {
		rank[string(s)] = 2 * i
	}
	for _, p := range phases() {
		rank[p.id] = rank[string(p.states[len(p.states)-1])] + 1
	}
	keys := make([]arrow, 0, len(arrows))
	for k := range arrows {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b arrow) int {
		if a.from != b.from {
			return rank[a.from] - rank[b.from]
		}
		return rank[string(a.to)] - rank[string(b.to)]
	})
	for _, k := range keys {
		tail, attrs, from := k.from, "", reach[State(k.from)]
		if g, ok := columns()[State(k.from)]; ok && columns()[k.to] == g {
			attrs = ", weight=20"
		}
		for _, p := range phases() {
			if p.id == k.from {
				tail, attrs, from = p.id+"_exit", fmt.Sprintf(", color=%q, fontcolor=%q", p.ink, p.ink), set{}
				for _, st := range p.states {
					for r := range reach[st] {
						from[r] = true
					}
				}
			}
		}
		// Letters only where a step is open to fewer regimes than both its source and its target.
		l := ""
		if len(arrows[k]) < len(from) && len(arrows[k]) < len(reach[k.to]) {
			l = arrows[k].String()
		}
		head := string(k.to)
		if k.to == StateClosed {
			// The end state is repeated beside each phase that can close, so no arrow crosses the figure to reach it;
			// a step open to fewer regimes names them on its marker.
			head = "closed_" + strings.ToLower(k.from)
			marker := "Closed"
			if l != "" {
				marker, l = "Closed ("+l+")", ""
			}
			fmt.Fprintf(&b, "  %s [label=%q, shape=box, style=\"rounded,filled\", fillcolor=\"gray25\", fontcolor=white, color=\"gray25\", fontsize=13];\n", head, marker)
			fmt.Fprintf(&b, "  { rank=same; %s; %s; }\n", tail, head)
		}
		fmt.Fprintf(&b, "  %s -> %s [xlabel=%q%s];\n", tail, head, l, attrs)
	}
	b.WriteString("}\n")
	return b.String()
}

// label title-cases a state's identifier, keeping SEPA and NQA as acronyms.
func label(id string) string {
	words := strings.Split(strings.ToLower(id), "_")
	for i, w := range words {
		if w == "sepa" || w == "nqa" {
			words[i] = strings.ToUpper(w)
		} else {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
