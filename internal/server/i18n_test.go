package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslate(t *testing.T) {
	assert.Equal(t, "Übersicht", translate("de", "Overview"))
	assert.Equal(t, "not in catalog", translate("de", "not in catalog"))
	assert.Equal(t, "Overview", translate("en", "Overview"))
}
