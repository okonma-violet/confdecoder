package confdecoder

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type ParsedFileData struct {
	parseddata        filedata
	Keys              []string // пишутся, даже если для ключей нет значения
	NestedStructsMode byte
}

type Row struct {
	Name  string
	Value string
}

// (Дефолтный режим) Значения полям вложенных структур присваиваются так же, как и обычным полям
const NestedStructsModeOne byte = 1

// Значения всем полям вложенной структуры присваиваются одновременно по имени поля-структуры, т.е. все значения в одну строку в файле, пропуск значения считается ошибкой
const NestedStructsModeTwo byte = 2

type filedata map[string]string

func (pfd *ParsedFileData) Rows() []Row {
	rows := make([]Row, 0, len(pfd.parseddata))
	for n, v := range pfd.parseddata {
		rows = append(rows, Row{n, v})
	}
	return rows
}

func ParseFile(filepath string) (*ParsedFileData, error) {
	rawdata, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	pfd := &ParsedFileData{parseddata: make(filedata), NestedStructsMode: 1}

	lines := strings.Split(string(rawdata), "\n")
	pfd.Keys = make([]string, 0, len(lines))

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		elems := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(elems) == 1 {
			if len(elems[0]) != 0 {
				pfd.Keys = append(pfd.Keys, elems[0])
			}
			continue
		}

		value := strings.TrimSpace(elems[1])
		if _, ok := pfd.parseddata[elems[0]]; ok {
			if len(value) != 0 {
				pfd.parseddata[elems[0]] = value
			}
		} else {
			pfd.Keys = append(pfd.Keys, elems[0])
			pfd.parseddata[elems[0]] = value
		}
	}

	if len(pfd.parseddata) == 0 {
		pfd.parseddata = nil // no err on empty file
	}
	return pfd, nil
}

func (pfd *ParsedFileData) DecodeTo(v ...interface{}) error {
	structsvalues := make([]reflect.Value, 0, len(v))

	for _, s := range v {
		pv := reflect.ValueOf(s)
		if pv.Kind() != reflect.Ptr {
			return errors.New("arg must be a pointer")
		}
		if pv.IsNil() {
			return errors.New("arg is nil")
		}
		sv := pv.Elem()
		if sv.Kind() != reflect.Struct {
			return errors.New("args must be a non-nil pointers to a structs")
		}
		if sv.NumField() != 0 {
			structsvalues = append(structsvalues, sv)
		}
	}

	for i := 0; i < len(structsvalues); i++ {
		l := structsvalues[i].NumField()

		for k := 0; k < l; k++ {

			fv := structsvalues[i].Field(k)
			fname := structsvalues[i].Type().Field(k).Name
			if !fv.CanSet() {
				continue
			}
		switching:
			switch fv.Kind() {
			case reflect.Ptr:
				if fv.IsNil() {
					fv.Set(reflect.New(fv.Type().Elem()))
				}
				fv = fv.Elem()
				goto switching
			case reflect.Struct:
				if fv.NumField() != 0 {
					if pfd.NestedStructsMode == NestedStructsModeOne {
						structsvalues = append(structsvalues, fv)
						continue
					}
				}
			case reflect.Interface:
				fv = fv.Elem()
				goto switching
			}
			if fv.IsValid() && fv.CanSet() {
				if err := pfd.parseddata.decodeToField(fname, fv); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func DecodeFile(filepath string, v ...interface{}) error {
	pfd, err := ParseFile(filepath)
	if err != nil {
		return err
	}
	return pfd.DecodeTo(v...)
}
func (data filedata) decodeToField(fieldname string, fv reflect.Value) error {
	if datv := data[fieldname]; datv != "" {
		//vv := reflect.ValueOf(datv)
		switch fv.Kind() {
		case reflect.Struct:
			if fv.NumField() == 0 {
				return nil
			}
			datvs := strings.Split(datv, " ")
			for i := 0; i < len(datvs); i++ {
				if len(datvs[i]) == 0 {
					datvs = datvs[:i+copy(datvs[i:], datvs[i+1:])]
				}
			}
			if fv.NumField() != len(datvs) {
				return errors.New("mismatch of num of values in file with num of fields in given struct " + fieldname)
			}

			var ffv reflect.Value
			for i := 0; i < len(datvs); i++ {
				ffv = fv.Field(i)
				if !ffv.IsValid() || !ffv.CanSet() {
					continue
				}
				if ffv.Type().Kind() == reflect.Ptr {
					if fv.IsNil() {
						ffv.Set(reflect.New(ffv.Type().Elem()))
					}
					ffv = ffv.Elem()
				}
				switch ffv.Kind() {
				case reflect.String:
					ffv.SetString(datvs[i])
				case reflect.Int:
					if convint, err := strconv.Atoi(datvs[i]); err == nil {
						covint64 := int64(convint)
						if ffv.OverflowInt(covint64) {
							return errors.New("value " + datvs[i] + " of field " + fieldname + " overflows int")
						}
						ffv.SetInt(covint64)
					} else {
						return errors.New("cant convert value " + datvs[i] + " of field " + fieldname + " to int, err: " + err.Error())
					}
				case reflect.Slice:
					switch ffv.Type().Elem().Kind() {
					case reflect.String:
						elems := strings.Split(strings.Trim(datvs[i], "{}[]"), ",")
						if len(elems) == 1 && len(strings.TrimSpace(elems[0])) == 0 {
							elems = nil
						}
						ffv.Set(reflect.MakeSlice(ffv.Type(), len(elems), len(elems)))
						for k := 0; k < len(elems); k++ {
							ffv.Index(k).SetString(elems[k])
						}
					case reflect.Int:
						elems := strings.Split(strings.Trim(datvs[i], "{}[]"), ",")
						if len(elems) == 1 && len(strings.TrimSpace(elems[0])) == 0 {
							elems = nil
						}
						ffv.Set(reflect.MakeSlice(ffv.Type(), len(elems), len(elems)))
						for k := 0; k < len(elems); k++ {
							if convint, err := strconv.Atoi(elems[k]); err == nil {
								covint64 := int64(convint)
								if ffv.Index(k).OverflowInt(covint64) {
									return errors.New("value " + elems[k] + " of field " + fieldname + " overflows int")
								}
								ffv.Index(k).SetInt(covint64)
							} else {
								return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
							}
						}
					default:
						return errors.New("unsupportable slice " + ffv.Type().Elem().Kind().String() + " type of field \"" + fieldname + "\"")
					}
				default:
					return errors.New("unsupportable type " + ffv.Kind().String() + " for a nested struct's field")
				}
			}

		case reflect.String:
			fv.SetString(datv)
		case reflect.Int:
			if convint, err := strconv.Atoi(datv); err == nil {
				covint64 := int64(convint)
				if fv.OverflowInt(covint64) {
					return errors.New("value of field " + fieldname + " overflows int")
				}
				fv.SetInt(covint64)
			} else {
				return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
			}
		case reflect.Slice:
			switch fv.Type().Elem().Kind() {
			case reflect.String:
				// if vv.Kind() == reflect.String {
				// 	fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
				// 	fv.Index(0).Set(vv)
				// 	return nil
				// }
				// fv.Set(reflect.MakeSlice(fv.Type(), vv.Len(), vv.Len()))
				// for i := 0; i < fv.Len(); i++ {
				// 	fv.Index(i).Set(vv.Index(i))
				// }
				elems := strings.Split(strings.Trim(datv, "{}[]"), ",")
				if len(elems) == 1 && len(strings.TrimSpace(elems[0])) == 0 {
					elems = nil
				}
				fv.Set(reflect.MakeSlice(fv.Type(), len(elems), len(elems)))
				for k := 0; k < len(elems); k++ {
					fv.Index(k).SetString(strings.TrimSpace(elems[k]))
				}

			case reflect.Int:
				// if vv.Kind() == reflect.String {
				// 	if convint, err := strconv.Atoi(vv.String()); err == nil {
				// 		covint64 := int64(convint)
				// 		if fv.Index(0).OverflowInt(covint64) {
				// 			return errors.New("value of field " + fieldname + " overflows int")
				// 		}
				// 		fv.Set(reflect.MakeSlice(fv.Type(), 1, 1))
				// 		fv.Index(0).SetInt(covint64)
				// 		return nil
				// 	} else {
				// 		return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
				// 	}
				// }
				// fv.Set(reflect.MakeSlice(fv.Type(), vv.Len(), vv.Len()))
				// for i := 0; i < fv.Len(); i++ {
				// 	if convint, err := strconv.Atoi(vv.Index(i).String()); err == nil {
				// 		covint64 := int64(convint)
				// 		if fv.Index(0).OverflowInt(covint64) {
				// 			return errors.New("value of field " + fieldname + " overflows int")
				// 		}
				// 		fv.Index(i).SetInt(covint64)
				// 	} else {
				// 		return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
				// 	}
				// }
				elems := strings.Split(strings.Trim(datv, "{}[]"), ",")
				if len(elems) == 1 && len(strings.TrimSpace(elems[0])) == 0 {
					elems = nil
				}
				fv.Set(reflect.MakeSlice(fv.Type(), len(elems), len(elems)))
				for k := 0; k < len(elems); k++ {
					if convint, err := strconv.Atoi(strings.TrimSpace(elems[k])); err == nil {
						covint64 := int64(convint)
						if fv.Index(k).OverflowInt(covint64) {
							return errors.New("value " + elems[k] + " of field " + fieldname + " overflows int")
						}
						fv.Index(k).SetInt(covint64)
					} else {
						return errors.New("cant convert value of field " + fieldname + " to int, err: " + err.Error())
					}
				}
			default:
				return errors.New("unsupportable slice " + fv.Type().Elem().Kind().String() + " type of field \"" + fieldname + "\"")
			}

		default:
			return errors.New("unsupportable type " + fv.Kind().String() + " of field \"" + fieldname + "\"")
		}
	}
	return nil
}
