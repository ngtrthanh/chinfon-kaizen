package app

import (
    "archive/zip"
    "bytes"
    "errors"
    "os"
    "path"
    "strings"
    "unicode/utf8"
)

// validateAttachmentFile checks file content independently of filenames and
// client Content-Type. It does not replace antivirus scanning.
func validateAttachmentFile(filePath, ext string) error {
    b, err := os.ReadFile(filePath) // caller caps upload at 10 MiB
    if err != nil { return err }
    if len(b) == 0 { return errors.New("empty attachment") }
    ok := false
    switch strings.ToLower(ext) {
    case ".pdf": ok = bytes.HasPrefix(b, []byte("%PDF-"))
    case ".png": ok = bytes.HasPrefix(b, []byte{137,80,78,71,13,10,26,10})
    case ".jpg", ".jpeg": ok = bytes.HasPrefix(b, []byte{0xff,0xd8,0xff})
    case ".gif": ok = bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a"))
    case ".webp": ok = len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP"
    case ".txt", ".csv": ok = utf8.Valid(b) && !bytes.ContainsRune(b, 0)
    case ".docx", ".xlsx": ok = validateOfficePackage(b, strings.ToLower(ext)) == nil
    }
    if !ok { return errors.New("invalid file signature or package") }
    return nil
}

func validateOfficePackage(b []byte, ext string) error {
    z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
    if err != nil { return err }
    if len(z.File) == 0 || len(z.File) > 512 { return errors.New("invalid package file count") }
    hasTypes, hasMain := false, false
    var size uint64
    for _, f := range z.File {
        n := strings.ReplaceAll(f.Name, "\\", "/")
        clean := path.Clean(n)
        if strings.HasPrefix(n,"/") || clean == ".." || strings.HasPrefix(clean,"../") || clean != n {
            return errors.New("unsafe zip path")
        }
        lower := strings.ToLower(n)
        if strings.Contains(lower,"vbaproject") || strings.HasSuffix(lower,".exe") ||
           strings.HasSuffix(lower,".dll") || strings.HasSuffix(lower,".js") {
            return errors.New("executable or macro in Office package")
        }
        size += f.UncompressedSize64
        if size > 40<<20 { return errors.New("oversized uncompressed package") }
        if n == "[Content_Types].xml" { hasTypes = true }
        if ext == ".docx" && n == "word/document.xml" { hasMain = true }
        if ext == ".xlsx" && n == "xl/workbook.xml" { hasMain = true }
    }
    if !hasTypes || !hasMain { return errors.New("invalid Office structure") }
    return nil
}
