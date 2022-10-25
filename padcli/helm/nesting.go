package helm

// Set exec values gets called when --exec <qualified_symbol> is passed in during
// jetpack dev.
func setSDKExecValues(appValues map[string]any, qualifiedSymbol string) {
	SetNestedField(appValues, "jetpack", "runSDKExec", true)
	SetNestedField(appValues, "jetpack", "qualifiedSymbol", qualifiedSymbol)
	SetNestedField(appValues, "jetpack", "runSDKRegister", false)
}

func SetNestedField(values map[string]any, field1 string, field2 string, value any) {
	ensureFieldIsMap(values, field1)
	values[field1].(map[string]any)[field2] = value
}

func setNestedFieldPath(values map[string]any, fp []string, value any) {
	if len(fp) == 0 {
		return
	} else if len(fp) == 1 {
		values[fp[0]] = value
		return
	}
	ensureFieldIsMap(values, fp[0])
	setNestedFieldPath(values[fp[0]].(map[string]any), fp[1:], value)
}

func ensureFieldIsMap(values map[string]any, field string) {
	if _, ok := values[field].(map[string]any); !ok {
		values[field] = map[string]any{}
	}
}
