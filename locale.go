// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"slices"
	"strings"
)

// Windows' language, keyboard layout and time zone in the guest's terms: a glibc locale,
// an XKB layout (with a variant) and an IANA time zone. The guest checks each against
// what Debian has and keeps its own (British English, a US keyboard, UTC) where one does
// not fit, so a mapping missing here costs a setting, never the setup.

// keyboards maps Windows keyboard layout IDs (KLID, as GetKeyboardLayoutName names them)
// to XKB, and names each for the setup's list.
var keyboards = []struct{ klid, xkb, name string }{
	{"00000409", "us", "English (US)"},
	{"00020409", "us(intl)", "English (US, international)"},
	{"00000809", "gb", "English (UK)"},
	{"00000452", "gb(extd)", "English (UK, extended)"},
	{"00001809", "ie", "Irish"},
	{"00000407", "de", "Deutsch"},
	{"00000807", "ch", "Deutsch (Schweiz)"},
	{"0000040C", "fr", "Français"},
	{"0000080C", "be", "Français (Belgique)"},
	{"00000813", "be", "Nederlands (België)"},
	{"0000100C", "ch(fr)", "Français (Suisse)"},
	{"00001009", "ca", "Français (Canada)"},
	{"00011009", "ca(multix)", "Canadian Multilingual"},
	{"00000413", "nl", "Nederlands"},
	{"00000410", "it", "Italiano"},
	{"0000040A", "es", "Español"},
	{"0000080A", "latam", "Español (Latinoamérica)"},
	{"00000816", "pt", "Português"},
	{"00000416", "br", "Português (Brasil)"},
	{"00010416", "br", "Português (Brasil, ABNT2)"},
	{"00000406", "dk", "Dansk"},
	{"00000414", "no", "Norsk"},
	{"0000041D", "se", "Svenska"},
	{"0000040B", "fi", "Suomi"},
	{"0000040F", "is", "Íslenska"},
	{"00000425", "ee", "Eesti"},
	{"00000426", "lv", "Latviešu"},
	{"00010427", "lt", "Lietuvių"},
	{"00000415", "pl", "Polski"},
	{"00010415", "pl(qwertz)", "Polski (214)"},
	{"00000405", "cz", "Čeština"},
	{"00010405", "cz(qwerty)", "Čeština (QWERTY)"},
	{"0000041B", "sk", "Slovenčina"},
	{"0000040E", "hu", "Magyar"},
	{"00000424", "si", "Slovenščina"},
	{"0000041A", "hr", "Hrvatski"},
	{"0000081A", "rs(latin)", "Srpski (latinica)"},
	{"00000C1A", "rs", "Српски"},
	{"00010418", "ro(std)", "Română"},
	{"00000418", "ro", "Română (legacy)"},
	{"00000402", "bg", "Български"},
	{"00000408", "gr", "Ελληνικά"},
	{"0000041F", "tr", "Türkçe (Q)"},
	{"0001041F", "tr(f)", "Türkçe (F)"},
	{"00000419", "ru", "Русский"},
	{"00000422", "ua", "Українська"},
	{"00000423", "by", "Беларуская"},
	{"0000040D", "il", "עברית"},
	{"00000401", "ara", "العربية"},
	{"0000041E", "th", "ไทย"},
	{"0000042A", "vn", "Tiếng Việt"},
	{"00000411", "jp", "日本語"},
	{"00000412", "kr", "한국어"},
}

// keyboardFor is the XKB keyboard for a KLID, US where none is known. A layout chosen
// with an input method (Chinese, for one) has the language in its high half and the
// keyboard in its low half, which is US on most of them.
func keyboardFor(klid string) string {
	klid = strings.ToUpper(klid)
	for _, k := range keyboards {
		if k.klid == klid {
			return k.xkb
		}
	}
	return "us"
}

// keyboardChoices lists the keyboards for the setup, each XKB keyboard once.
func keyboardChoices() []choice {
	var list []choice
	for _, k := range keyboards {
		if !slices.ContainsFunc(list, func(c choice) bool { return c.ID == k.xkb }) {
			list = append(list, choice{k.xkb, k.name})
		}
	}
	return list
}

// locales are the languages the setup lists, each in its own words.
var locales = []choice{
	{"en_GB.UTF-8", "English (United Kingdom)"},
	{"en_US.UTF-8", "English (United States)"},
	{"en_IE.UTF-8", "English (Ireland)"},
	{"de_DE.UTF-8", "Deutsch (Deutschland)"},
	{"de_AT.UTF-8", "Deutsch (Österreich)"},
	{"de_CH.UTF-8", "Deutsch (Schweiz)"},
	{"fr_FR.UTF-8", "Français (France)"},
	{"fr_BE.UTF-8", "Français (Belgique)"},
	{"fr_CH.UTF-8", "Français (Suisse)"},
	{"fr_CA.UTF-8", "Français (Canada)"},
	{"nl_NL.UTF-8", "Nederlands"},
	{"nl_BE.UTF-8", "Nederlands (België)"},
	{"it_IT.UTF-8", "Italiano"},
	{"es_ES.UTF-8", "Español"},
	{"pt_PT.UTF-8", "Português"},
	{"pt_BR.UTF-8", "Português (Brasil)"},
	{"da_DK.UTF-8", "Dansk"},
	{"nb_NO.UTF-8", "Norsk bokmål"},
	{"sv_SE.UTF-8", "Svenska"},
	{"fi_FI.UTF-8", "Suomi"},
	{"et_EE.UTF-8", "Eesti"},
	{"lv_LV.UTF-8", "Latviešu"},
	{"lt_LT.UTF-8", "Lietuvių"},
	{"pl_PL.UTF-8", "Polski"},
	{"cs_CZ.UTF-8", "Čeština"},
	{"sk_SK.UTF-8", "Slovenčina"},
	{"hu_HU.UTF-8", "Magyar"},
	{"sl_SI.UTF-8", "Slovenščina"},
	{"hr_HR.UTF-8", "Hrvatski"},
	{"ro_RO.UTF-8", "Română"},
	{"bg_BG.UTF-8", "Български"},
	{"el_GR.UTF-8", "Ελληνικά"},
	{"tr_TR.UTF-8", "Türkçe"},
	{"uk_UA.UTF-8", "Українська"},
	{"ru_RU.UTF-8", "Русский"},
	{"ja_JP.UTF-8", "日本語"},
	{"ko_KR.UTF-8", "한국어"},
	{"zh_CN.UTF-8", "中文（中国）"},
	{"zh_TW.UTF-8", "中文（台灣）"},
}

// localeFor turns a Windows locale name (de-DE, sr-Latn-RS) into a glibc one
// (de_DE.UTF-8); British English where the name has no language and country.
func localeFor(name string) string {
	parts := strings.Split(name, "-")
	lang, country := strings.ToLower(parts[0]), strings.ToUpper(parts[len(parts)-1])
	if len(parts) < 2 || len(lang) < 2 || len(lang) > 3 || len(country) != 2 {
		return "en_GB.UTF-8"
	}
	return lang + "_" + country + ".UTF-8"
}

// localeChoices lists the languages for the setup, with Windows' own first if the list
// lacks it.
func localeChoices(current string) []choice {
	if slices.ContainsFunc(locales, func(c choice) bool { return c.ID == current }) {
		return locales
	}
	return append([]choice{{current, strings.TrimSuffix(current, ".UTF-8")}}, locales...)
}

// timeZoneFallback maps the Windows time zones of Europe and North America to IANA ones
// for a Windows without its ICU (before Windows 10 1903), which knows them all.
var timeZoneFallback = map[string]string{
	"UTC":                            "Etc/UTC",
	"GMT Standard Time":              "Europe/London",
	"W. Europe Standard Time":        "Europe/Berlin",
	"Romance Standard Time":          "Europe/Paris",
	"Central Europe Standard Time":   "Europe/Budapest",
	"Central European Standard Time": "Europe/Warsaw",
	"GTB Standard Time":              "Europe/Bucharest",
	"FLE Standard Time":              "Europe/Kyiv",
	"E. Europe Standard Time":        "Europe/Chisinau",
	"Russian Standard Time":          "Europe/Moscow",
	"Turkey Standard Time":           "Europe/Istanbul",
	"Eastern Standard Time":          "America/New_York",
	"Central Standard Time":          "America/Chicago",
	"Mountain Standard Time":         "America/Denver",
	"Pacific Standard Time":          "America/Los_Angeles",
}

// timeZones are the time zones the setup suggests; any IANA name may be typed.
var timeZones = []string{
	"Europe/Amsterdam", "Europe/Athens", "Europe/Berlin", "Europe/Brussels",
	"Europe/Bucharest", "Europe/Budapest", "Europe/Copenhagen", "Europe/Dublin",
	"Europe/Helsinki", "Europe/Istanbul", "Europe/Kyiv", "Europe/Lisbon", "Europe/London",
	"Europe/Luxembourg", "Europe/Madrid", "Europe/Oslo", "Europe/Paris", "Europe/Prague",
	"Europe/Riga", "Europe/Rome", "Europe/Sofia", "Europe/Stockholm", "Europe/Tallinn",
	"Europe/Vienna", "Europe/Vilnius", "Europe/Warsaw", "Europe/Zagreb", "Europe/Zurich",
	"America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
	"America/Toronto", "America/Sao_Paulo", "Asia/Tokyo", "Asia/Seoul", "Asia/Shanghai",
	"Asia/Kolkata", "Australia/Sydney", "Etc/UTC",
}

// loginName is a login name made of a Windows user name: small letters, digits, - and
// _, starting with a letter; empty when none is left.
func loginName(name string) string {
	if _, after, ok := strings.Cut(name, `\`); ok {
		name = after // DOMAIN\user
	}
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', b.Len() > 0 && (r >= '0' && r <= '9' || r == '-' || r == '_'):
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) > 32 {
		s = s[:32]
	}
	if !userPattern.MatchString(s) || s == "root" || s == "live" {
		return ""
	}
	return s
}
