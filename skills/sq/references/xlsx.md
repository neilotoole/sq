# Excel XLSX (`xlsx` driver)

Microsoft [XLSX](https://www.microsoft.com/microsoft-365/excel) workbooks. **Read-only** document source.

**Canonical docs:** [XLSX (Excel)](https://sq.io/docs/drivers/xlsx/)

## Supported formats

Supported extensions: `.xlsx`, `.xlam`, `.xlsm`, `.xltm`, `.xltx`. The driver type for these supported formats remains **`xlsx`**. Older **`.xls` / `.xlsb`** files are unsupported and will not work with this driver.

## Add a source

```shell
sq add ./workbook.xlsx
sq add --driver=xlsx ./workbook.xlsx
```

`sq` can usually [detect](https://sq.io/docs/detect/#driver-type) the format.

## Worksheets

Each **sheet** is a separate table: `@handle.sheetname`.

## Header rows

Per-sheet header detection; use [`--ingest.header`](https://sq.io/docs/config/#ingestheader) when detection is wrong (applies to **all** sheets in the workbook—see caveats on [sq.io](https://sq.io/docs/drivers/xlsx/)).

## Document source

Excel is [ingested/cached](https://sq.io/docs/source#document-source) like other document drivers.
