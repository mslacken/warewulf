package node

import (
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/hpcng/warewulf/internal/pkg/util"
)

type sortByName []NodeConf

func (a sortByName) Len() int           { return len(a) }
func (a sortByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortByName) Less(i, j int) bool { return a[i].id < a[j].id }
func GetUnsetVerbs() []string {
	return []string{"UNSET", "DELETE", "UNDEF", "undef", "--", "nil", "0.0.0.0"}
}

/**********
 *
 * Filters
 *
 *********/

/*
Filter a given slice of NodeConf against a given
regular expression
*/
func FilterByName(set []NodeConf, searchList []string) []NodeConf {
	var ret []NodeConf
	unique := make(map[string]NodeConf)

	if len(searchList) > 0 {
		for _, search := range searchList {
			for _, entry := range set {
				if match, _ := regexp.MatchString("^"+search+"$", entry.id); match {
					unique[entry.id] = entry
				}
			}
		}
		for _, n := range unique {
			ret = append(ret, n)
		}
	} else {
		ret = set
	}

	sort.Sort(sortByName(ret))
	return ret
}

/*
Filter a given map of NodeConf against given regular expression.
*/
func FilterMapByName(inputMap map[string]*NodeConf, searchList []string) (retMap map[string]*NodeConf) {
	retMap = map[string]*NodeConf{}
	if len(searchList) > 0 {
		for _, search := range searchList {
			for name, nConf := range inputMap {
				if match, _ := regexp.MatchString("^"+search+"$", name); match {
					retMap[name] = nConf
				}
			}
		}
	}
	return retMap
}

func NewConf(id string) (nodeconf NodeConf) {
	nodeconf.Ipmi = new(IpmiConf)
	nodeconf.Ipmi.Tags = map[string]string{}
	nodeconf.Kernel = new(KernelConf)
	nodeconf.NetDevs = make(map[string]*NetDevs)
	nodeconf.Tags = map[string]string{}
	nodeconf.id = id
	return nodeconf
}

/*
Check if the Netdev is empty, so has no values set
*/
func ObjectIsEmpty(obj interface{}) bool {
	if obj == nil {
		return true
	}
	varType := reflect.TypeOf(obj)
	varVal := reflect.ValueOf(obj)
	if varType.Kind() == reflect.Ptr && !varVal.IsNil() {
		return ObjectIsEmpty(varVal.Elem().Interface())
	}
	if varVal.IsZero() {
		return true
	}
	for i := 0; i < varType.NumField(); i++ {
		if varType.Field(i).Type.Kind() == reflect.String && !varVal.Field(i).IsZero() {
			val := varVal.Field(i).Interface().(string)
			if val != "" {
				return false
			}
		} else if varType.Field(i).Type == reflect.TypeOf(map[string]string{}) {
			if len(varVal.Field(i).Interface().(map[string]string)) != 0 {
				return false
			}
		} else if varType.Field(i).Type.Kind() == reflect.Ptr {
			if !ObjectIsEmpty(varVal.Field(i).Interface()) {
				return false
			}
		}
	}
	return true
}

/*
Flattens out a NodeConf, which means if there are no explicit values in *IpmiConf
or *KernelConf, these pointer will set to nil. This will remove something like
ipmi: {} from nodes.conf
*/
func (info *NodeConf) Flatten() {
	recursiveFlatten(info)
}
func recursiveFlatten(strct interface{}) {
	confType := reflect.TypeOf(strct)
	confVal := reflect.ValueOf(strct)
	for j := 0; j < confType.Elem().NumField(); j++ {
		if confVal.Elem().Field(j).Type().Kind() == reflect.Ptr && !confVal.Elem().Field(j).IsNil() {
			// iterate now over the ptr fields
			setToNil := true
			nestedType := reflect.TypeOf(confVal.Elem().Field(j).Interface())
			nestedVal := reflect.ValueOf(confVal.Elem().Field(j).Interface())
			for i := 0; i < nestedType.Elem().NumField(); i++ {
				if nestedType.Elem().Field(i).Type.Kind() == reflect.String &&
					nestedVal.Elem().Field(i).Interface().(string) != "" {
					setToNil = false
				} else if nestedType.Elem().Field(i).Type == reflect.TypeOf([]string{}) &&
					len(nestedVal.Elem().Field(i).Interface().([]string)) != 0 {
					setToNil = false
				} else if nestedType.Elem().Field(i).Type == reflect.TypeOf(map[string]string{}) &&
					len(nestedVal.Elem().Field(i).Interface().(map[string]string)) != 0 {
					setToNil = false
				}
			}
			if setToNil {
				confVal.Elem().Field(j).Set(reflect.Zero(confVal.Elem().Field(j).Type()))
			}
		}
	}
}

/*
Create a string slice, where every element represents a yaml entry, used for node/profile edit
in order to get a summary of all available elements
*/
func UnmarshalConf(obj interface{}, excludeList []string) (lines []string) {
	objType := reflect.TypeOf(obj)
	// now iterate of every field
	for i := 0; i < objType.NumField(); i++ {
		if objType.Field(i).Tag.Get("comment") != "" {
			if ymlStr, ok := getYamlString(objType.Field(i), excludeList); ok {
				lines = append(lines, ymlStr...)
			}
		}
		if objType.Field(i).Type.Kind() == reflect.Ptr && objType.Field(i).Tag.Get("yaml") != "" {
			typeLine := objType.Field(i).Tag.Get("yaml")
			if len(strings.Split(typeLine, ",")) > 1 {
				typeLine = strings.Split(typeLine, ",")[0] + ":"
			}
			lines = append(lines, typeLine)
			nestedLine := UnmarshalConf(reflect.New(objType.Field(i).Type.Elem()).Elem().Interface(), excludeList)
			for _, ln := range nestedLine {
				lines = append(lines, "  "+ln)
			}
		} else if objType.Field(i).Type.Kind() == reflect.Map && objType.Field(i).Type.Elem().Kind() == reflect.Ptr {
			typeLine := objType.Field(i).Tag.Get("yaml")
			if len(strings.Split(typeLine, ",")) > 1 {
				typeLine = strings.Split(typeLine, ",")[0] + ":"
			}
			lines = append(lines, typeLine, "  element:")
			nestedLine := UnmarshalConf(reflect.New(objType.Field(i).Type.Elem().Elem()).Elem().Interface(), excludeList)
			for _, ln := range nestedLine {
				lines = append(lines, "    "+ln)
			}
		}
	}
	return lines
}

/*
Get the string of the yaml tag
*/
func getYamlString(myType reflect.StructField, excludeList []string) ([]string, bool) {
	ymlStr := myType.Tag.Get("yaml")
	if len(strings.Split(ymlStr, ",")) > 1 {
		ymlStr = strings.Split(ymlStr, ",")[0]
	}
	if util.InSlice(excludeList, ymlStr) {
		return []string{""}, false
	} else if myType.Tag.Get("comment") == "" && myType.Type.Kind() == reflect.String {
		return []string{""}, false
	}
	if myType.Type.Kind() == reflect.String {
		fieldType := myType.Tag.Get("type")
		if fieldType == "" {
			fieldType = "string"
		}
		ymlStr += ": " + fieldType
		return []string{ymlStr}, true
	} else if myType.Type == reflect.TypeOf([]string{}) {
		return []string{ymlStr + ":", "  - string"}, true
	} else if myType.Type == reflect.TypeOf(map[string]string{}) {
		return []string{ymlStr + ":", "  key: value"}, true
	} else if myType.Type.Kind() == reflect.Ptr {
		return []string{ymlStr + ":"}, true
	}
	return []string{ymlStr}, true
}

/*
Getters for unexported fields
*/

/*
Returns the id of the node/profile
*/
func (node *NodeConf) Id() string {
	return node.id
}

/*
Returns if the node is a valid in the database
*/
func (node *NodeConf) Valid() bool {
	return node.valid
}

/*
Check if the netdev is the primary one
*/
func (dev *NetDevs) Primary() bool {
	return dev.primary
}
