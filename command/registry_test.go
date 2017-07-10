package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandRegistry(t *testing.T) {
	assert := assert.New(t)

	r := newCommandRegistry()
	assert.NotNil(r.cmds)
	assert.NotNil(r.mu)
	assert.NotNil(evgRegistry)

	assert.Len(r.cmds, 0)

	factory := CommandFactory(func() (Command, bool) { return nil, true })
	assert.NotNil(factory)
	assert.Error(r.registerCommand("", factory))
	assert.Len(r.cmds, 0)
	assert.Error(r.registerCommand("foo", nil))
	assert.Len(r.cmds, 0)

	assert.NoError(r.registerCommand("cmd.factory", factory))
	assert.Len(r.cmds, 1)
	assert.Error(r.registerCommand("cmd.factory", factory))
	assert.Len(r.cmds, 1)

	retFactory, ok := r.getCommandFactory("cmd.factory")
	assert.True(ok)
	assert.NotNil(retFactory)
}
