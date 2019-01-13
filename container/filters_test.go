package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whefter/watchtower/container/mocks"
)

func TestBuildWatchtowerContainersFilter(t *testing.T) {
	var tag = "foo"

	filter := BuildWatchtowerContainersFilter(tag)

	container := new(mocks.FilterableContainer)
	container.On("IsWatchtower").Maybe().Return(false)
	container.On("WatchtowerTag").Return("not"+tag, false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("IsWatchtower").Maybe().Return(false)
	container.On("WatchtowerTag").Return(tag, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("IsWatchtower").Maybe().Return(true)
	container.On("WatchtowerTag").Return("not"+tag, false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("IsWatchtower").Maybe().Return(true)
	container.On("WatchtowerTag").Return(tag, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}

func TestBuildTagFilter(t *testing.T) {
	var tag = "foo"

	filter := BuildTagFilter(tag)

	container := new(mocks.FilterableContainer)
	container.On("WatchtowerTag").Return("not"+tag, false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("WatchtowerTag").Return("not"+tag, true)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("WatchtowerTag").Return(tag, false)
	assert.False(t, filter(container))
	container.AssertExpectations(t)

	container = new(mocks.FilterableContainer)
	container.On("WatchtowerTag").Return(tag, true)
	assert.True(t, filter(container))
	container.AssertExpectations(t)
}
