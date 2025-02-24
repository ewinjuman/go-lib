package convert

import (
	"bufio"
	"encoding/json"
	"github.com/ewinjuman/go-lib/v2/utils/convert/eng"
	"math"
	"os"
	"strings"
	"time"
)

// convert object to another object
func ObjectToObject(in interface{}, out interface{}) {
	dataByte, _ := json.Marshal(in)
	json.Unmarshal(dataByte, &out)
}

// convert object to string
func ObjectToString(data interface{}) string {
	dataByte, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(dataByte)
}

// convert string to object
func StringToObject(in string, out interface{}) {
	json.Unmarshal([]byte(in), &out)
	return
}

// Read Perlines from file
func ReadLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func ConvertDateID(date time.Time, toFormat string) string {
	const (
		formatShortA   = "02-Jan-2006"
		formatShort    = "02-Jan-2006"
		formatShortAlt = "2-Jan-2006"
		formatLong     = "02 January 2006"
		formatLongAlt  = "2 January 2006"
	)

	// Replace patterns for short month names
	shortMonthReplacer := strings.NewReplacer(
		"May", "Mei",
		"Aug", "Agu",
		"Oct", "Okt",
		"Dec", "Des",
	)

	// Replace patterns for full month names
	longMonthReplacer := strings.NewReplacer(
		"January", "Januari",
		"February", "Februari",
		"March", "Maret",
		"April", "April",
		"May", "Mei",
		"June", "Juni",
		"July", "Juli",
		"August", "Agustus",
		"September", "September",
		"October", "Oktober",
		"November", "November",
		"December", "Desember",
	)

	result := date.Format(toFormat)

	switch {
	case strings.Contains(toFormat, formatShort) || strings.Contains(toFormat, formatShortAlt):
		return shortMonthReplacer.Replace(result)
	case strings.Contains(toFormat, formatLong) || strings.Contains(toFormat, formatLongAlt):
		return longMonthReplacer.Replace(result)
	}

	return result
}

const (
	groupsNumber int = 12
)

var _smallNumbers = []string{
	"nol", "satu", "dua", "tiga", "empat",
	"lima", "enam", "tujuh", "delapan", "sembilan",
	"sepuluh", "sebelas", "dua belas", "tiga belas", "empat belas",
	"lima belas", "enam belas", "tujuh belas", "delapan belas", "sembilan belas",
}

var _scaleNumbers = []string{
	"", "ribu", "juta", "miliyar", "triliun",
	"kuadriliun", "kuintiliun", "sekstilion", "septiliun", "oktiliun",
	"noniliun", "desiliun",
}

type digitGroup int

// Convert converts number into the words representation.
func ConvertToWord(number int) string {
	return convert(number)
}

// Convert converts number into the words representation.
func ConvertEng(number int) string {
	return eng.Convert(number, false)
}

// ConvertAnd converts number into the words representation
// with " and " added between number groups.
func ConvertEngAnd(number int) string {
	return eng.Convert(number, true)
}

// ConvertAnd converts number into the words representation
// with " and " added between number groups.
//func ConvertAnd(number int) string {
//	return convert(number, true)
//}

func convert(number int) string {
	// Zero rule
	if number == 0 {
		return _smallNumbers[0]
	}

	// Divide into three-digits group
	var groups [groupsNumber]digitGroup
	positive := math.Abs(float64(number))

	// Form three-digit groups
	for i := 0; i < groupsNumber; i++ {
		groups[i] = digitGroup(math.Mod(positive, 1000))
		positive /= 1000
	}

	var textGroup [groupsNumber]string
	for i := 0; i < groupsNumber; i++ {
		textGroup[i] = digitGroup2Text(groups[i])
	}
	combined := textGroup[0]

	for i := 1; i < groupsNumber; i++ {
		if groups[i] != 0 {
			prefix := textGroup[i] + " " + _scaleNumbers[i]
			if prefix == "satu ribu" {
				prefix = "seribu"
			}

			if len(combined) != 0 {
				prefix += " "
			}
			combined = prefix + combined
		}

	}

	if number < 0 {
		combined = "minus " + combined
	}
	return combined
}

func intMod(x, y int) int {
	return int(math.Mod(float64(x), float64(y)))
}

func digitGroup2Text(group digitGroup) (ret string) {
	hundreds := group / 100
	tensUnits := intMod(int(group), 100)

	if hundreds != 0 && hundreds != 1 {
		ret += _smallNumbers[hundreds] + " ratus"

		if tensUnits != 0 {
			ret += " "
		}
	} else if hundreds == 1 {
		ret += "seratus"
		if tensUnits != 0 {
			ret += " "
		}
	}

	tens := tensUnits / 10
	units := intMod(tensUnits, 10)

	if tens >= 2 {
		ret += _smallNumbers[tens] + " puluh"
		if units != 0 {
			ret += " " + _smallNumbers[units]
		}
	} else if tensUnits != 0 {
		ret += _smallNumbers[tensUnits]
	}

	return
}

//
//func ConvertPhoneNumber(mobilePhoneNumber string) (newMobilePhoneNumber string, err error) {
//	phoneNumber := strings.Replace(mobilePhoneNumber, " ", "", -1)
//	if strings.HasPrefix(phoneNumber, "62") {
//		newMobilePhoneNumber = strings.Replace(phoneNumber, "62", "0", 1)
//	} else if strings.HasPrefix(phoneNumber, "+62") {
//		println(phoneNumber)
//		newMobilePhoneNumber = strings.Replace(phoneNumber, "+62", "0", 1)
//	} else if strings.HasPrefix(phoneNumber, "0") {
//		newMobilePhoneNumber = phoneNumber
//	} else {
//		newMobilePhoneNumber = "0"+phoneNumber
//	}
//	valid := validation.Validation{}
//	if v := valid.Numeric(newMobilePhoneNumber, "Mobile Phone Number"); !v.Ok {
//		println(newMobilePhoneNumber)
//		println(v.error.Message)
//		err = error.New(http.StatusBadRequest, "FAILED", "Mobile Phone Number Tidak Valid")
//		return
//	}
//
//	return
//}
