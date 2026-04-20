#!/usr/bin/env bash
# Generate CSS for light and dark mode syntax highlighting.
# Although this theme is based on Doks (https://getdoks.org)
# which uses highlight.js by default, we chose to use Hugo's
# built-in Chroma instead.
set -e

# Chroma Style Gallery: https://xyproto.github.io/splash/docs/all.html
theme_light=rose-pine-dawn
theme_light=nord
#theme_light=solarized-light



#theme_dark=rose-pine
#theme_dark=monokai
theme_dark=nord

# Generate light mode
./node_modules/.bin/hugo/hugo gen chromastyles --style=$theme_light > ./assets/scss/components/_syntax.scss

# For dark mode, we need a little hack:
echo "[data-dark-mode] body { " > ./assets/scss/components/_syntax-dark.scss
./node_modules/.bin/hugo/hugo gen chromastyles --style=$theme_dark >> ./assets/scss/components/_syntax-dark.scss
echo "}" >> ./assets/scss/components/_syntax-dark.scss

