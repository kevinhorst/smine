package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/kevinhorst/smine/internal/plans"
)

// planSectionsOk opens the first output line; fdesign's title states match the line it starts.
const planSectionsOk = "plan-sections: ok"

func planSectionsLine(gate string) string {
	return planSectionsOk + " — " + gate + " gate"
}

func runPlanSections(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: rules plan-sections <plan file>")
		return exitError
	}

	content, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "rules:", err)
		return exitError
	}

	markdown := string(content)
	drivers := plans.ParseDrivers(markdown)
	unit := plans.ParseChanges(markdown)
	violations := plans.CheckPlan(drivers, unit)
	hasEntries := unit != nil && len(unit.Entries) > 0
	isAddsChecked := plans.IsAddsChecked(args[0])
	var decisions []*plans.Decision
	var phaseViolations []plans.Violation
	if isAddsChecked {
		decisions = plans.ParseDecisions(markdown)
		violations = append(violations, plans.CheckDecisions(decisions)...)
	}
	if isAddsChecked && hasEntries {
		phaseViolations = plans.CheckPhases(unit)
		violations = append(violations, phaseViolations...)
	}
	isOrderChecked := unit != nil && plans.IsOrderChecked(args[0])
	if isOrderChecked {
		violations = append(violations, plans.CheckOrder(unit)...)
	}

	for _, violation := range violations {
		fmt.Printf("%s:%d: %s\n", args[0], violation.Line, violation.Message)
	}
	if len(phaseViolations) > 0 {
		fmt.Println(plans.ExpectedPhaseTable(unit))
	}
	if len(violations) > 0 {
		return exitViolations
	}

	fmt.Println(planSectionsLine(plans.Gate(markdown)))
	if isAddsChecked {
		fmt.Println(plans.AddsLine(decisions))
	}
	if hasEntries {
		fmt.Println(strings.Join(plans.DriverLines(drivers, unit), "\n"))
	}
	return exitClean
}
