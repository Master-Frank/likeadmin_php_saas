package export

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"strings"
)

func writeXLSX(path string, records [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	files := []struct {
		name string
		body string
	}{
		{"[Content_Types].xml", xlsxContentTypes},
		{"_rels/.rels", xlsxRels},
		{"xl/workbook.xml", xlsxWorkbook},
		{"xl/_rels/workbook.xml.rels", xlsxWorkbookRels},
		{"xl/styles.xml", xlsxStyles},
	}
	for _, file := range files {
		w, err := zw.Create(file.name)
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := w.Write([]byte(file.body)); err != nil {
			_ = zw.Close()
			return err
		}
	}
	w, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		_ = zw.Close()
		return err
	}
	if err := writeSheet(w, records); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func writeSheet(w io.Writer, records [][]string) error {
	if _, err := io.WriteString(w, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`); err != nil {
		return err
	}
	if _, err := io.WriteString(w, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`); err != nil {
		return err
	}
	for i, rec := range records {
		row := i + 1
		style := 0
		if i == 0 {
			style = 1
		}
		if _, err := fmt.Fprintf(w, `<row r="%d">`, row); err != nil {
			return err
		}
		for j, cell := range rec {
			ref := colName(j) + fmt.Sprintf("%d", row)
			if style > 0 {
				if _, err := fmt.Fprintf(w, `<c r="%s" t="inlineStr" s="%d"><is><t xml:space="preserve">%s</t></is></c>`, ref, style, xmlEscape(cell)); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, `<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, ref, xmlEscape(cell)); err != nil {
					return err
				}
			}
		}
		if _, err := io.WriteString(w, `</row>`); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, `</sheetData></worksheet>`)
	return err
}

func colName(idx int) string {
	name := ""
	for idx >= 0 {
		name = string(rune('A'+idx%26)) + name
		idx = idx/26 - 1
	}
	return name
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

const xlsxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`

const xlsxRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const xlsxWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>
</workbook>`

const xlsxWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`

const xlsxStyles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="2">
<font><sz val="11"/><name val="Calibri"/></font>
<font><sz val="11"/><color rgb="FFFFFFFF"/><name val="Calibri"/></font>
</fonts>
<fills count="2">
<fill><patternFill patternType="none"/></fill>
<fill><patternFill patternType="solid"><fgColor rgb="FF00B0F0"/><bgColor indexed="64"/></patternFill></fill>
</fills>
<borders count="2">
<border><left/><right/><top/><bottom/><diagonal/></border>
<border>
<left style="thin"/><right style="thin"/><top style="thin"/><bottom style="thin"/><diagonal/>
</border>
</borders>
<cellXfs count="2">
<xf fontId="0" fillId="0" borderId="1"/>
<xf fontId="1" fillId="1" borderId="1" applyFont="1" applyFill="1"/>
</cellXfs>
</styleSheet>`
