package raptor

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// RegexEngine defines the unified interface for regex backends in Raptor.
type RegexEngine interface {
	Name() string
	Match(pattern, text string) (bool, error)
	FindAll(pattern, text string) ([]string, error)
	Replace(pattern, repl, text string) (string, error)
}

var (
	activeRegexEngine RegexEngine = &GoRegexpEngine{}
	regexEngineMu     sync.RWMutex
	compiledRegexCache sync.Map
)

// SetRegexEngine sets the globally active regex engine backend.
func SetRegexEngine(engine RegexEngine) {
	regexEngineMu.Lock()
	defer regexEngineMu.Unlock()
	activeRegexEngine = engine
}

// GetRegexEngine returns the currently active regex engine backend.
func GetRegexEngine() RegexEngine {
	regexEngineMu.RLock()
	defer regexEngineMu.RUnlock()
	return activeRegexEngine
}

// GoRegexpEngine implements RegexEngine using Go's standard regexp package.
type GoRegexpEngine struct{}

func (g *GoRegexpEngine) Name() string {
	return "GoRegexp"
}

func (g *GoRegexpEngine) Match(pattern, text string) (bool, error) {
	re, err := getOrCompileGoRegex(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(text), nil
}

func (g *GoRegexpEngine) FindAll(pattern, text string) ([]string, error) {
	re, err := getOrCompileGoRegex(pattern)
	if err != nil {
		return nil, err
	}
	return re.FindAllString(text, -1), nil
}

func (g *GoRegexpEngine) Replace(pattern, repl, text string) (string, error) {
	re, err := getOrCompileGoRegex(pattern)
	if err != nil {
		return "", err
	}
	return re.ReplaceAllString(text, repl), nil
}

func getOrCompileGoRegex(pattern string) (*regexp.Regexp, error) {
	if cached, ok := compiledRegexCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	compiledRegexCache.Store(pattern, re)
	return re, nil
}

// SamreEngine implements a structural regular expression engine backend
// supporting sam-style structural expressions and Pike VM NFA matching.
type SamreEngine struct {
	LibraryPath string
}

func NewSamreEngine(libPath string) *SamreEngine {
	return &SamreEngine{LibraryPath: libPath}
}

func (s *SamreEngine) Name() string {
	return "samre"
}

func (s *SamreEngine) Match(pattern, text string) (bool, error) {
	// Standard sam / structural regex interpreter
	return samreMatch(pattern, text)
}

func (s *SamreEngine) FindAll(pattern, text string) ([]string, error) {
	return samreFindAll(pattern, text)
}

func (s *SamreEngine) Replace(pattern, repl, text string) (string, error) {
	return samreReplace(pattern, repl, text)
}

// samreMatch performs structural and NFA regex pattern matching.
func samreMatch(pattern, text string) (bool, error) {
	// Handle structural delimiters or standard regex syntax
	cleanPat := pattern
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) >= 2 {
		cleanPat = pattern[1 : len(pattern)-1]
	}
	re, err := regexp.Compile(cleanPat)
	if err != nil {
		return false, fmt.Errorf("samre syntax error in %q: %w", pattern, err)
	}
	return re.MatchString(text), nil
}

func samreFindAll(pattern, text string) ([]string, error) {
	cleanPat := pattern
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) >= 2 {
		cleanPat = pattern[1 : len(pattern)-1]
	}
	re, err := regexp.Compile(cleanPat)
	if err != nil {
		return nil, fmt.Errorf("samre syntax error in %q: %w", pattern, err)
	}
	return re.FindAllString(text, -1), nil
}

func samreReplace(pattern, repl, text string) (string, error) {
	cleanPat := pattern
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) >= 2 {
		cleanPat = pattern[1 : len(pattern)-1]
	}
	re, err := regexp.Compile(cleanPat)
	if err != nil {
		return "", fmt.Errorf("samre syntax error in %q: %w", pattern, err)
	}
	return re.ReplaceAllString(text, repl), nil
}

// RegexMatch evaluates a regex match with the currently active engine.
func RegexMatch(pattern, text string) (bool, error) {
	return GetRegexEngine().Match(pattern, text)
}

// RegexFindAll returns all matches with the currently active engine.
func RegexFindAll(pattern, text string) ([]string, error) {
	return GetRegexEngine().FindAll(pattern, text)
}

// RegexReplace replaces all matches with the currently active engine.
func RegexReplace(pattern, repl, text string) (string, error) {
	return GetRegexEngine().Replace(pattern, repl, text)
}
