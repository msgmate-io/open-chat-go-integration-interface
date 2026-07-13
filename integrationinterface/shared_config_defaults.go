package integrationinterface

import "encoding/json"

func ApplySharedConfigDefaults(current map[string]interface{}) map[string]interface{} {
	out := cloneJSONMap(current)
	if out == nil {
		out = map[string]interface{}{}
	}
	for _, def := range List() {
		if def.SharedConfigDefaults == nil {
			continue
		}
		defaults := cloneJSONMap(def.SharedConfigDefaults(cloneJSONMap(out)))
		if defaults == nil {
			continue
		}
		out = mergeMissingJSONMaps(out, defaults)
	}
	return out
}

func mergeMissingJSONMaps(current map[string]interface{}, defaults map[string]interface{}) map[string]interface{} {
	if current == nil {
		current = map[string]interface{}{}
	}
	if defaults == nil {
		return current
	}
	out := cloneJSONMap(current)
	for key, rawDefault := range defaults {
		existing, exists := out[key]
		if !exists {
			out[key] = cloneJSONValue(rawDefault)
			continue
		}
		existingMap, existingIsMap := existing.(map[string]interface{})
		defaultMap, defaultIsMap := rawDefault.(map[string]interface{})
		if existingIsMap && defaultIsMap {
			out[key] = mergeMissingJSONMaps(existingMap, defaultMap)
		}
	}
	return out
}

func cloneJSONMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		out := map[string]interface{}{}
		for key, value := range input {
			out[key] = cloneJSONValue(value)
		}
		return out
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]interface{}{}
	}
	return out
}

func cloneJSONValue(value interface{}) interface{} {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return value
	}
	return out
}
