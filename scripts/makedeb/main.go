package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
)

func main() {
	control := "Package: testpkg\nVersion: 1.0.0\nArchitecture: amd64\nMaintainer: Test <test@test>\nDescription: test package\n"
	var ctar bytes.Buffer
	gw := gzip.NewWriter(&ctar)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: "control", Mode: 0644, Size: int64(len(control))}
	if err := tw.WriteHeader(hdr); err != nil {
		panic(err)
	}
	if _, err := tw.Write([]byte(control)); err != nil {
		panic(err)
	}
	tw.Close()
	gw.Close()

	debdata := []byte("test binary content")
	var dtar bytes.Buffer
	gw2 := gzip.NewWriter(&dtar)
	tw2 := tar.NewWriter(gw2)
	hdr2 := &tar.Header{Name: "./usr/bin/testpkg", Mode: 0755, Size: int64(len(debdata))}
	if err := tw2.WriteHeader(hdr2); err != nil {
		panic(err)
	}
	if _, err := tw2.Write(debdata); err != nil {
		panic(err)
	}
	tw2.Close()
	gw2.Close()

	ctarBytes := ctar.Bytes()
	dtarBytes := dtar.Bytes()

	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	writeAr(&out, "debian-binary", []byte("2.0\n"))
	writeAr(&out, "control.tar.gz", ctarBytes)
	writeAr(&out, "data.tar.gz", dtarBytes)

	if err := os.WriteFile("/tmp/testpkg_1.0.0_amd64.deb", out.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Println("written /tmp/testpkg_1.0.0_amd64.deb", out.Len())
}

func writeAr(w *bytes.Buffer, name string, data []byte) {
	fmt.Fprintf(w, "%-16s", name)
	fmt.Fprintf(w, "%-12d", 0)
	fmt.Fprintf(w, "%-6d", 0)
	fmt.Fprintf(w, "%-6d", 0)
	fmt.Fprintf(w, "%-8o", 0644)
	fmt.Fprintf(w, "%-10d", len(data))
	w.WriteString("`\n")
	w.Write(data)
	if len(data)%2 == 1 {
		w.WriteByte('\n')
	}
}
