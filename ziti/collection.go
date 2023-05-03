package ziti

import (
	"github.com/michaelquigley/pfxlog"
	cmap "github.com/orcaman/concurrent-map/v2"
	"os"
	"strings"
)

// An SdkCollection allows Context instances to be instantiated and maintained as a group. Useful in scenarios
// where multiple Context instances are managed together. Instead of using ziti.NewContext() like functions, use
// the function provided on this type to automatically have contexts added as they are created. If ConfigTypes
// is set, they will be automatically added to any instantiated Context through `New*` functions.
type SdkCollection struct {
	contexts    cmap.ConcurrentMap[string, Context]
	ConfigTypes []string
}

// NewSdkCollection creates a new empty collection.
func NewSdkCollection() *SdkCollection {
	return &SdkCollection{
		contexts: cmap.New[Context](),
	}
}

// NewSdkCollectionFromEnv will create an empty SdkCollection and then attempt to populate it from configuration files
// provided in a semicolon separate list of file paths retrieved from an environment variable.
func NewSdkCollectionFromEnv(envVariable string) *SdkCollection {
	collection := NewSdkCollection()

	envValue := os.Getenv(envVariable)

	identityFiles := strings.Split(envValue, ";")

	for _, identityFile := range identityFiles {

		if identityFile == "" {
			continue
		}
		cfg, err := NewConfigFromFile(identityFile)

		if err != nil {
			pfxlog.Logger().Errorf("failed to load config from file '%s'", identityFile)
			continue
		}

		//collection.NewContext stores the new ctx in its internal collection
		_, err = collection.NewContext(cfg)

		if err != nil {
			pfxlog.Logger().Errorf("failed to create context from '%s'", identityFile)
			continue
		}
	}

	return collection
}

// Add allows the arbitrary idempotent inclusion of a Context in the current collection. If a Context with the same id
// as an existing Context is added and is a different instance, the original is closed and removed.
func (set *SdkCollection) Add(ctx Context) {
	set.contexts.Upsert(ctx.GetId(), ctx, func(exist bool, valueInMap Context, newValue Context) Context {
		if exist && valueInMap != nil && valueInMap != newValue {
			valueInMap.Close()
		}

		return newValue
	})

	set.contexts.Set(ctx.GetId(), ctx)
}

// Remove removes the supplied Context from the collection. It is not closed or altered in any way.
func (set *SdkCollection) Remove(ctx Context) {
	set.contexts.Remove(ctx.GetId())
}

// RemoveById removes a context by its string id.  It is not closed or altered in any way.
func (set *SdkCollection) RemoveById(id string) {
	set.contexts.Remove(id)
}

// ForAll call the provided function `f` on each Context.
func (set *SdkCollection) ForAll(f func(ctx Context) bool) {
	set.contexts.IterCb(func(key string, ctx Context) {
		_ = f(ctx)
	})
}

// NewContextFromFile is the same as ziti.NewContextFromFile but will also add the resulting
// context to the current collection.
func (set *SdkCollection) NewContextFromFile(file string) (Context, error) {
	return set.NewContextFromFileWithOpts(file, nil)
}

// NewContextFromFileWithOpts is the same as ziti.NewContextFromFileWithOpts but will also add
// the resulting context to the current collection.
func (set *SdkCollection) NewContextFromFileWithOpts(file string, options *Options) (Context, error) {
	cfg, err := NewConfigFromFile(file)

	if err != nil {
		return nil, err
	}

	return set.NewContextWithOpts(cfg, options)
}

// NewContext is the same as ziti.NewContext but will also add the resulting context to the current collection.
func (set *SdkCollection) NewContext(cfg *Config) (Context, error) {
	return set.NewContextWithOpts(cfg, nil)
}

// NewContextWithOpts is the same as ziti.NewContextWithOpts but will also add the resulting context to the current
// collection.
func (set *SdkCollection) NewContextWithOpts(cfg *Config, options *Options) (Context, error) {
	cfg.ConfigTypes = append(cfg.ConfigTypes, set.ConfigTypes...)

	ctx, err := NewContextWithOpts(cfg, options)

	if err != nil {
		return nil, err
	}

	set.Add(ctx)

	return ctx, nil
}
