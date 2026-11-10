package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/PRASHMANE/Go-ToDo/pkg/config"
)

func TestHelpers(t *testing.T) {
	t.Setenv("S", "value")
	t.Setenv("I", "42")
	t.Setenv("BAD_I", "x")
	t.Setenv("F", "2.5")
	t.Setenv("D", "15m")
	t.Setenv("L", " a, b ,,c ")

	assert.Equal(t, "value", config.String("S", "fb"))
	assert.Equal(t, "fb", config.String("MISSING", "fb"))
	assert.Equal(t, 42, config.Int("I", 1))
	assert.Equal(t, 1, config.Int("BAD_I", 1))
	assert.InDelta(t, 2.5, config.Float("F", 0), 0.0001)
	assert.Equal(t, 15*time.Minute, config.Duration("D", time.Second))
	assert.Equal(t, []string{"a", "b", "c"}, config.List("L", nil))
	assert.Equal(t, []string{"x"}, config.List("MISSING", []string{"x"}))

	_, err := config.Required("MISSING")
	assert.Error(t, err)
	v, err := config.Required("S")
	assert.NoError(t, err)
	assert.Equal(t, "value", v)
}
