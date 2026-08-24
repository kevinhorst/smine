package server

const languageGerman = "de"

// germanCatalog maps English source strings (the keys double as the English
// default) to their German overlay. A missing key renders the English
// original — the page never breaks (plan D5).
var germanCatalog = map[string]string{
	"Overview":  "Übersicht",
	"Sessions":  "Sitzungen",
	"Proposals": "Vorschläge",
	"Open":      "Offen",
	"Notes":     "Notizen",
	"all":       "alle",
	"batches":   "Auswertungen",
	"comment (rejection reason or agent instruction)": "Kommentar (Grund oder Hinweis)",
	"details":                "Details",
	"evidence":               "Belege",
	"fields":                 "Angaben",
	"postpone":               "später",
	"technical detail":       "Technisches Detail",
	"Arcs":                   "Verläufe",
	"Created:":               "Erstellt:",
	"Reload from disk":       "Neu laden",
	"Sessions analyzed":      "Ausgewertete Sitzungen",
	"Last analysis":          "Letzte Auswertung",
	"Frustration / Positive": "Frust / Positiv",
	"newest batch":           "neueste Auswertung",
	"No notes.":              "Keine Notizen.",
	"No proposal JSONs found (proposals/*.json).": "Noch keine Vorschläge vorhanden.",
	"No session data found.":                      "Noch keine Auswertungen vorhanden.",
}

func translate(language, text string) string {
	if language != languageGerman {
		return text
	}
	if translated, ok := germanCatalog[text]; ok {
		return translated
	}
	return text
}
