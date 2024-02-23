package oncecache

// Opt is an option for [New].
type Opt interface {
	// optioner is a marker method to unify our two functional option types,
	// optApplier and concreteOptApplier.
	optioner()
}

// optApplier is an [Opt] that uses the apply method to configure the fields of
// [Cache]. It must be type-parameterized, as this Opt access the parameterized
// fields of [Cache].
type optApplier[K comparable, V any] interface {
	Opt
	apply(c *Cache[K, V])
}

// concreteOptApplier is an [Opt] type that uses the applyConcrete method to
// configure the non-parameterized (concrete) fields of [Cache].
//
// TODO: Write a post about this pattern:
// "Mixing concrete and type-parameterized functional options".
type concreteOptApplier interface {
	Opt
	applyConcrete(c *concreteCache)
}

// concreteCache contains pointers to the non-parameterized (concrete) state of
// [Cache]. It is passed to concreteOptApplier.applyConcrete by [New].
type concreteCache struct {
	name *string
}

var _ concreteOptApplier = (*Name)(nil)

// Name is an [Opt] for [New] that sets the cache's name. The name is accessible
// via [Cache.Name].
//
//	c := oncecache.New[int, string](fetch, oncecache.Name("foobar"))
//
// The name is used by [Cache.String] and [Cache.LogValue]. If [Name] is not
// specified, a random name such as "cache-38a2b7d4" is generated.
type Name string

func (o Name) applyConcrete(c *concreteCache) {
	*c.name = string(o)
}

func (o Name) optioner() {}
