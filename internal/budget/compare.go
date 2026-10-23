package budget

import "math"

// Agreement is the verdict over one value pair.
type Agreement string

const (
	AgreementAgree  Agreement = "agree"
	AgreementDiffer Agreement = "differ"
	AgreementSingle Agreement = "single"

	costRelativeTolerance  = 0.05
	tokenRelativeTolerance = 0.01
	wallAbsoluteSeconds    = 60
)

// Wall time compares by absolute difference, the other metrics by relative difference.
var (
	CostTolerance  = Tolerance{Relative: costRelativeTolerance}
	TokenTolerance = Tolerance{Relative: tokenRelativeTolerance}
	WallTolerance  = Tolerance{Absolute: wallAbsoluteSeconds}
)

// Paired sums cover only sessions carrying both sides, so a missing source never produces a difference.
type Pair struct {
	Both          int
	Ccusage       float64
	PairedCcusage float64
	PairedPeek    float64
	Peek          float64
	Single        int
}

// Add counts one session; a side with hasValue false contributes nothing.
func (p *Pair) Add(ccusage float64, hasCcusage bool, hasPeek bool, peek float64) {
	if hasCcusage {
		p.Ccusage += ccusage
	}
	if hasPeek {
		p.Peek += peek
	}
	switch {
	case hasCcusage && hasPeek:
		p.Both++
		p.PairedCcusage += ccusage
		p.PairedPeek += peek
	case hasCcusage, hasPeek:
		p.Single++
	}
}

func (p *Pair) Agreement(tolerance Tolerance) Agreement {
	if p.Both == 0 {
		return AgreementSingle
	}

	difference := math.Abs(p.PairedCcusage - p.PairedPeek)
	if tolerance.Absolute > 0 {
		return agreementOf(difference <= tolerance.Absolute)
	}

	base := math.Max(p.PairedCcusage, p.PairedPeek)
	if base == 0 {
		return AgreementAgree
	}
	return agreementOf(difference/base <= tolerance.Relative)
}

func (p *Pair) Merge(other *Pair) {
	p.Both += other.Both
	p.Ccusage += other.Ccusage
	p.PairedCcusage += other.PairedCcusage
	p.PairedPeek += other.PairedPeek
	p.Peek += other.Peek
	p.Single += other.Single
}

// A relative tolerance compares against the larger value.
type Tolerance struct {
	Absolute float64
	Relative float64
}

func agreementOf(isWithin bool) Agreement {
	if isWithin {
		return AgreementAgree
	}
	return AgreementDiffer
}

// Any differ wins, then agree; single only when nothing was compared.
func Worst(verdicts []Agreement) Agreement {
	worst := AgreementSingle
	for _, verdict := range verdicts {
		switch verdict {
		case AgreementDiffer:
			return AgreementDiffer
		case AgreementAgree:
			worst = AgreementAgree
		case AgreementSingle:
		}
	}
	return worst
}
