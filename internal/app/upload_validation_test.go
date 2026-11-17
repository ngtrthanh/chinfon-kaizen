package app

import (
    "archive/zip"
    "bytes"
    "os"
    "path/filepath"
    "testing"
)

func TestAttachmentRejectsSpoofedContent(t *testing.T) {
    dir := t.TempDir()
    for _, tc := range []struct{ext, content string; valid bool}{
        {".pdf","not a PDF",false},
        {".pdf","%PDF-1.7\n%%EOF",true},
        {".txt","hello",true},
        {".txt","hello\x00bad",false},
        {".png","fake image",false},
        {".zip","PKxxxx",false},
    }{
        name := filepath.Join(dir,"file"+tc.ext)
        if err:=os.WriteFile(name,[]byte(tc.content),0600);err!=nil{t.Fatal(err)}
        good:=validateAttachmentFile(name,tc.ext)==nil
        if good!=tc.valid{t.Fatalf("%s accepted=%v, want %v",tc.ext,good,tc.valid)}
    }
}
func TestOfficeMacroAndPackageValidation(t *testing.T) {
    makeZip := func(macro bool) []byte {
        var b bytes.Buffer
        w:=zip.NewWriter(&b)
        for _, n:=range []string{"[Content_Types].xml","word/document.xml"}{
            f,_:=w.Create(n); _,_=f.Write([]byte("<xml/>"))
        }
        if macro { f,_:=w.Create("word/vbaProject.bin");_,_=f.Write([]byte("danger")) }
        _=w.Close()
        return b.Bytes()
    }
    dir:=t.TempDir()
    p:=filepath.Join(dir,"doc.docx")
    if err:=os.WriteFile(p,makeZip(false),0600);err!=nil{t.Fatal(err)}
    if err:=validateAttachmentFile(p,".docx");err!=nil{t.Fatalf("valid docx rejected: %v",err)}
    if err:=os.WriteFile(p,makeZip(true),0600);err!=nil{t.Fatal(err)}
    if err:=validateAttachmentFile(p,".docx");err==nil{t.Fatal("macro package accepted")}
}
