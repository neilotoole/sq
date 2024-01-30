// Package cobraz contains supplemental logic for dealing with spf13/cobra.
package cobraz

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// GenCompletionScriptsCmdName is the name of the cobra built-in "completion"
// command that generates shell completion scripts.
// Note that this is not the same as the hidden "__complete" command that
// actually returns the completion results. That's cobra.ShellCompRequestCmd
// (and also cobra.ShellCompNoDescRequestCmd).
const GenCompletionScriptsCmdName = "completion"

// Defines the text values for cobra.ShellCompDirective.
const (
	ShellCompDirectiveErrorText         = "ShellCompDirectiveError"
	ShellCompDirectiveNoSpaceText       = "ShellCompDirectiveNoSpace"
	ShellCompDirectiveNoFileCompText    = "ShellCompDirectiveNoFileComp"
	ShellCompDirectiveFilterFileExtText = "ShellCompDirectiveFilterFileExt"
	ShellCompDirectiveFilterDirsText    = "ShellCompDirectiveFilterDirs"
	ShellCompDirectiveKeepOrderText     = "ShellCompDirectiveKeepOrder"
	ShellCompDirectiveDefaultText       = "ShellCompDirectiveDefault"
	ShellCompDirectiveUnknownText       = "ShellCompDirectiveUnknown"
)

// ParseDirectivesLine parses the line of text returned by "__complete" cmd
// that contains the text description of the result.
// The line looks like:
//
//	Completion ended with directive: ShellCompDirectiveNoSpace, ShellCompDirectiveKeepOrder
//
// Note that this function will panic on an unknown directive.
func ParseDirectivesLine(directivesLine string) []cobra.ShellCompDirective {
	trimmedLine := strings.TrimPrefix(strings.TrimSpace(directivesLine), "Completion ended with directive: ")
	parts := strings.Split(trimmedLine, ", ")
	directives := make([]cobra.ShellCompDirective, 0, len(parts))
	for _, part := range parts {
		switch part {
		case ShellCompDirectiveErrorText:
			directives = append(directives, cobra.ShellCompDirectiveError)
		case ShellCompDirectiveNoSpaceText:
			directives = append(directives, cobra.ShellCompDirectiveNoSpace)
		case ShellCompDirectiveNoFileCompText:
			directives = append(directives, cobra.ShellCompDirectiveNoFileComp)
		case ShellCompDirectiveFilterFileExtText:
			directives = append(directives, cobra.ShellCompDirectiveFilterFileExt)
		case ShellCompDirectiveFilterDirsText:
			directives = append(directives, cobra.ShellCompDirectiveFilterDirs)
		case ShellCompDirectiveKeepOrderText:
			directives = append(directives, cobra.ShellCompDirectiveKeepOrder)
		case ShellCompDirectiveDefaultText:
			directives = append(directives, cobra.ShellCompDirectiveDefault)
		default:
			panic(fmt.Sprintf("Unknown cobra.ShellCompDirective {%s} in: %s", part, directivesLine))
		}
	}
	return directives
}

// ExtractDirectives extracts the individual directives
// from a combined directive.
func ExtractDirectives(result cobra.ShellCompDirective) []cobra.ShellCompDirective {
	if result == cobra.ShellCompDirectiveDefault {
		return []cobra.ShellCompDirective{cobra.ShellCompDirectiveDefault}
	}

	var a []cobra.ShellCompDirective

	allDirectives := []cobra.ShellCompDirective{
		cobra.ShellCompDirectiveError,
		cobra.ShellCompDirectiveNoSpace,
		cobra.ShellCompDirectiveNoFileComp,
		cobra.ShellCompDirectiveFilterFileExt,
		cobra.ShellCompDirectiveFilterDirs,
		cobra.ShellCompDirectiveKeepOrder,
		cobra.ShellCompDirectiveDefault,
	}

	for _, directive := range allDirectives {
		if directive&result > 0 {
			a = append(a, directive)
		}
	}

	return a
}

// MarshalDirective marshals a cobra.ShellCompDirective to text strings,
// after extracting the embedded directives.
func MarshalDirective(directive cobra.ShellCompDirective) []string {
	gotDirectives := ExtractDirectives(directive)

	s := make([]string, len(gotDirectives))
	for i, d := range gotDirectives {
		switch d {
		case cobra.ShellCompDirectiveError:
			s[i] = ShellCompDirectiveErrorText
		case cobra.ShellCompDirectiveNoSpace:
			s[i] = ShellCompDirectiveNoSpaceText
		case cobra.ShellCompDirectiveNoFileComp:
			s[i] = ShellCompDirectiveNoFileCompText
		case cobra.ShellCompDirectiveFilterFileExt:
			s[i] = ShellCompDirectiveFilterFileExtText
		case cobra.ShellCompDirectiveFilterDirs:
			s[i] = ShellCompDirectiveFilterDirsText
		case cobra.ShellCompDirectiveKeepOrder:
			s[i] = ShellCompDirectiveKeepOrderText
		case cobra.ShellCompDirectiveDefault:
			s[i] = ShellCompDirectiveDefaultText
		default:
			// Should never happen
			s[i] = ShellCompDirectiveUnknownText
		}
	}

	return s
}
