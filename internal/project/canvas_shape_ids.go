package project

import (
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type canvasShapeIDRepair struct {
	oldID string
	newID string
}

type canvasShapeIDFieldFix struct {
	id    string
	field string
	value string
}

func ensureCanvasShapeIDs(jsonStr string, result *CanvasRepairResult) (string, error) {
	store := gjson.Get(jsonStr, canvasStorePath)
	if !store.Exists() {
		return jsonStr, nil
	}

	existing := map[string]bool{}
	store.ForEach(func(key, value gjson.Result) bool {
		existing[key.String()] = true
		return true
	})

	var repairs []canvasShapeIDRepair
	idAliases := map[string]string{}
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() != "shape" {
			return true
		}
		oldID := key.String()
		recordID := value.Get("id").String()
		if !canonicalCanvasShapeID(oldID) {
			newID, err := uniqueCanvasShapeID(existing)
			if err != nil {
				repairs = append(repairs, canvasShapeIDRepair{oldID: oldID, newID: ""})
				return true
			}
			repairs = append(repairs, canvasShapeIDRepair{oldID: oldID, newID: newID})
			idAliases[oldID] = newID
			if recordID != "" {
				idAliases[recordID] = newID
			}
			return true
		}
		if recordID != "" && recordID != oldID {
			idAliases[recordID] = oldID
		}
		return true
	})

	for _, repair := range repairs {
		if repair.newID == "" {
			return "", fmt.Errorf("generate normalized shape id for %s", repair.oldID)
		}
		record := gjson.Get(jsonStr, canvasStorePath+"."+repair.oldID).Raw
		if record == "" {
			continue
		}
		record, err := sjson.Set(record, "id", repair.newID)
		if err != nil {
			return "", fmt.Errorf("normalize shape id %s: %w", repair.oldID, err)
		}
		jsonStr, err = sjson.SetRaw(jsonStr, canvasStorePath+"."+repair.newID, record)
		if err != nil {
			return "", fmt.Errorf("insert normalized shape id %s: %w", repair.newID, err)
		}
		jsonStr, err = sjson.Delete(jsonStr, canvasStorePath+"."+repair.oldID)
		if err != nil {
			return "", fmt.Errorf("delete old shape id %s: %w", repair.oldID, err)
		}
		result.Changed = true
		result.NormalizedShapeIDs++
	}

	store = gjson.Get(jsonStr, canvasStorePath)
	var fixes []canvasShapeIDFieldFix
	store.ForEach(func(key, value gjson.Result) bool {
		if value.Get("typeName").String() != "shape" {
			return true
		}
		id := key.String()
		if value.Get("id").String() != id {
			fixes = append(fixes, canvasShapeIDFieldFix{id: id, field: "id", value: id})
		}
		if newParentID, ok := idAliases[value.Get("parentId").String()]; ok {
			fixes = append(fixes, canvasShapeIDFieldFix{id: id, field: "parentId", value: newParentID})
		}
		return true
	})

	for _, fix := range fixes {
		var err error
		jsonStr, err = sjson.Set(jsonStr, canvasStorePath+"."+fix.id+"."+fix.field, fix.value)
		if err != nil {
			return "", fmt.Errorf("normalize shape %s %s: %w", fix.id, fix.field, err)
		}
		result.Changed = true
		if fix.field == "id" {
			result.NormalizedShapeIDs++
		}
	}

	return jsonStr, nil
}

func uniqueCanvasShapeID(existing map[string]bool) (string, error) {
	for {
		id, err := newShapeID()
		if err != nil {
			return "", err
		}
		if !existing[id] {
			existing[id] = true
			return id, nil
		}
	}
}
