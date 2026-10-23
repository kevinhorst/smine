package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/plans"
)

// planClaimsSchema is the schema number of the printed ledger.
const planClaimsSchema = 1

// planClaim is one refute-ledger row the fdesign refute workflow takes verbatim.
type planClaim struct {
	ClaimId  string `json:"claim_id"`
	Evidence string `json:"evidence"`
	Source   string `json:"source"`
	Text     string `json:"text"`
}

type planClaimsDocument struct {
	Claims []planClaim `json:"claims"`
	Schema int         `json:"schema"`
}

// restingDecisions lists the decisions whose Facts cell links the assumption.
func restingDecisions(decisions []*plans.Decision, id string) []string {
	link := "[" + id + "]("
	var ids []string
	for _, decision := range decisions {
		if strings.Contains(decision.Facts, link) {
			ids = append(ids, decision.Id)
		}
	}
	return ids
}

func runPlanClaims(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: rules plan-claims <plan file>")
		return exitError
	}

	content, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	markdown := string(content)
	decisions := plans.ParseDecisions(markdown)
	assumptions := plans.ParseAssumptions(markdown)
	document := &planClaimsDocument{Claims: make([]planClaim, 0, len(assumptions)), Schema: planClaimsSchema}
	for _, assumption := range assumptions {
		resting := restingDecisions(decisions, assumption.Id)
		claim := planClaim{
			ClaimId:  assumption.Id,
			Evidence: assumption.Location,
			Source:   assumption.Assumption + " — decisions: " + strings.Join(resting, ", "),
			Text:     assumption.Reality,
		}
		document.Claims = append(document.Claims, claim)
	}
	return printJson(document)
}
