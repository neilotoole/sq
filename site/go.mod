module github.com/neilotoole/sq/site

go 1.19

require cj.rs/gohugo-asciinema v0.0.0-20260303220240-6d3214cdecb7 // indirect

// The upstream module declares its path as the cj.rs vanity import, but that
// domain no longer serves a go-import meta tag, so module resolution (and the
// Hugo/Netlify site build) fails. Redirect fetches to the GitHub source, which
// hosts the identical module.
replace cj.rs/gohugo-asciinema => github.com/cljoly/gohugo-asciinema v0.0.0-20260303220240-6d3214cdecb7
