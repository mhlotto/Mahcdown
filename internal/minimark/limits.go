package minimark

import (
	"errors"
	"fmt"
)

const (
	// DefaultMaxSourceBytes limits Markdown input to 8 MiB.
	DefaultMaxSourceBytes = 8 * 1024 * 1024
	// DefaultMaxNestingDepth limits recursive parser nesting. The document root is depth 0.
	DefaultMaxNestingDepth = 64
	// DefaultMaxParseItems limits nodes and retained parser-created structure.
	DefaultMaxParseItems = 200_000
	// DefaultMaxASTNodes is retained for source compatibility. Use DefaultMaxParseItems.
	DefaultMaxASTNodes = DefaultMaxParseItems
)

var (
	ErrSourceSizeLimit   = errors.New("Markdown source size limit exceeded")
	ErrNestingDepthLimit = errors.New("Markdown nesting depth limit exceeded")
	ErrParseItemLimit    = errors.New("Markdown parse-item limit exceeded")
	// ErrASTNodeLimit is retained for errors.Is compatibility. Use ErrParseItemLimit.
	ErrASTNodeLimit  = ErrParseItemLimit
	ErrInvalidLimits = errors.New("invalid parser limits")
)

// Limits configures parser safety boundaries. Zero fields use the safe defaults.
type Limits struct {
	MaxSourceBytes  int
	MaxNestingDepth int
	MaxParseItems   int
	// MaxASTNodes is retained for source compatibility. Use MaxParseItems.
	MaxASTNodes int
}

type LimitError struct {
	Kind  error
	Limit int
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%v (limit %d)", e.Kind, e.Limit)
}

func (e *LimitError) Unwrap() error { return e.Kind }

func normalizedLimits(limits Limits) (Limits, error) {
	if limits.MaxSourceBytes < 0 || limits.MaxNestingDepth < 0 || limits.MaxParseItems < 0 || limits.MaxASTNodes < 0 {
		return Limits{}, ErrInvalidLimits
	}
	if limits.MaxParseItems != 0 && limits.MaxASTNodes != 0 && limits.MaxParseItems != limits.MaxASTNodes {
		return Limits{}, ErrInvalidLimits
	}
	if limits.MaxParseItems == 0 && limits.MaxASTNodes != 0 {
		limits.MaxParseItems = limits.MaxASTNodes
	}
	if limits.MaxSourceBytes == 0 {
		limits.MaxSourceBytes = DefaultMaxSourceBytes
	}
	if limits.MaxNestingDepth == 0 {
		limits.MaxNestingDepth = DefaultMaxNestingDepth
	}
	if limits.MaxParseItems == 0 {
		limits.MaxParseItems = DefaultMaxParseItems
	}
	limits.MaxASTNodes = limits.MaxParseItems
	return limits, nil
}

type parserState struct {
	limits Limits
	nodes  int
	err    error
}

func (state *parserState) consumeItem() bool {
	if state.err != nil {
		return false
	}
	if state.nodes >= state.limits.MaxParseItems {
		state.err = &LimitError{Kind: ErrParseItemLimit, Limit: state.limits.MaxParseItems}
		return false
	}
	state.nodes++
	return true
}

func (state *parserState) allowDepth(depth int) bool {
	if state.err != nil {
		return false
	}
	if depth > state.limits.MaxNestingDepth {
		state.err = &LimitError{Kind: ErrNestingDepthLimit, Limit: state.limits.MaxNestingDepth}
		return false
	}
	return true
}
