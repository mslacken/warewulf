package node

import (
	"reflect"
)

/*
struct to hold the fields of GetFields
*/
type NodeFields struct {
	Field  string
	Source string
	Value  string
}

/*
Get all the info out of NodeConf. If emptyFields is set true, all fields
are shown not only the ones with effective values
*/
func (nodeYml *NodeYaml) GetFields(node NodeConf, emptyFields bool) (output []NodeFields, err error) {
	fieldMap := make(map[string]NodeFields)
	for _, p := range node.Profiles {
		if profile, ok := nodeYml.NodeProfiles[p]; ok {
			recursiveFields(profile, emptyFields, "", &fieldMap, p)
		}
	}
	recursiveFields(node, emptyFields, "", &fieldMap, "")
	return output, nil
}

/*
Internal function which travels through all fields of a NodeConf and for this
reason needs tb called via interface{}
*/
func recursiveFields(obj interface{}, emtyFields bool, prefix string,
	fieldMap *map[string]NodeFields, source string) {
	valObj := reflect.ValueOf(obj)
	typeObj := reflect.TypeOf(obj)
	for i := 0; i < typeObj.Elem().NumField(); i++ {
		if valObj.Elem().Field(i).IsValid() && valObj.Elem().Field(i).String() != "" {
			output = append(output, NodeFields{
				Field:  prefix + typeObj.Elem().Field(i).Name,
				Source: myField.Source(),
				Value:  myField.Print(),
			})

		} else if typeObj.Elem().Field(i).Type == reflect.TypeOf(map[string]*Entry{}) {
			for key, val := range valObj.Elem().Field(i).Interface().(map[string]*Entry) {
				if emptyFields || val.Get() != "" {
					output = append(output, NodeFields{
						Field:  prefix + typeObj.Elem().Field(i).Name + "[" + key + "]",
						Source: val.Source(),
						Value:  val.Print(),
					})
				}
			}
			if valObj.Elem().Field(i).Len() == 0 && emptyFields {
				output = append(output, NodeFields{
					Field: prefix + typeObj.Elem().Field(i).Name + "[]",
				})
			}
		} else if typeObj.Elem().Field(i).Type.Kind() == reflect.Map {
			mapIter := valObj.Elem().Field(i).MapRange()
			for mapIter.Next() {
				nestedOut := recursiveFields(mapIter.Value().Interface(), emptyFields, prefix+typeObj.Elem().Field(i).Name+"["+mapIter.Key().String()+"].")
				if len(nestedOut) == 0 {
					output = append(output, NodeFields{
						Field: prefix + typeObj.Elem().Field(i).Name + "[" + mapIter.Key().String() + "]",
					})
				} else {
					output = append(output, nestedOut...)
				}
			}
			if valObj.Elem().Field(i).Len() == 0 && emptyFields {
				output = append(output, NodeFields{
					Field: prefix + typeObj.Elem().Field(i).Name + "[]",
				})
			}
		} else if typeObj.Elem().Field(i).Type.Kind() == reflect.Ptr {
			nestedOut := recursiveFields(valObj.Elem().Field(i).Interface(), emptyFields, prefix+typeObj.Elem().Field(i).Name+".")
			output = append(output, nestedOut...)
		}
	}
	return
}
