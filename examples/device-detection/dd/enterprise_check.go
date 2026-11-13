/* *********************************************************************
 * This Original Work is copyright of 51 Degrees Mobile Experts Limited.
 * Copyright 2026 51 Degrees Mobile Experts Limited, Davidson House,
 * Forbury Square, Reading, Berkshire, United Kingdom RG1 3EU.
 *
 * This Original Work is licensed under the European Union Public Licence
 * (EUPL) v.1.2 and is subject to its terms as set out below.
 *
 * If a copy of the EUPL was not distributed with this file, You can obtain
 * one at https://opensource.org/licenses/EUPL-1.2.
 *
 * The 'Compatible Licences' set out in the Appendix to the EUPL (as may be
 * amended by the European Commission) shall be deemed incompatible for
 * the purposes of the Work and the provisions of the compatibility
 * clause in Article 5 of the EUPL shall not apply.
 *
 * If using the Work as, or as part of, a network application, by
 * including the attribution notice(s) required under Article 5 of the EUPL
 * in the end user terms of the application under an appropriate heading,
 * such notice(s) shall fulfill the requirements of that article.
 * ********************************************************************* */

package dd

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// htmlTags matches the tags of a web page, which CheckEnterprise leaves out
// so that a page is read as its text.
var htmlTags = regexp.MustCompile(`<[^>]*>`)

// contactUsLink is the start of every example's link to ask for a copy of the
// Enterprise data file, which each example ends with its own utm_content and
// utm_term.
const contactUsLink = "https://51degrees.com/contact-us?" +
	"utm_source=code&utm_medium=example&utm_campaign=51degrees-go"

// CheckEnterprise returns an error unless the output of an example, as text
// or as a web page, shows each of EnterpriseProperties with a value or as
// needing the Enterprise data file, and gives the link to ask for a copy when
// one needs it. The examples' tests call it, so it holds with either file.
func CheckEnterprise(output string) error {
	// A page's link is in a tag, so it is looked for in the whole page.
	text := html.UnescapeString(htmlTags.ReplaceAllString(output, ""))
	needs := false
	for _, property := range EnterpriseProperties {
		switch {
		case strings.Contains(text, property+": "+NeedsEnterprise):
			needs = true
		case !strings.Contains(text, property+": "):
			return fmt.Errorf("the output does not show %s", property)
		}
	}
	if needs && !strings.Contains(html.UnescapeString(output), contactUsLink) {
		return fmt.Errorf("the output shows a property that needs the "+
			"Enterprise data file without the link to ask for a copy, "+
			"which starts %s", contactUsLink)
	}
	return nil
}
