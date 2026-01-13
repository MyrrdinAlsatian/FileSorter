// Package detector declares custom file signature matchers wired into github.com/h2non/filetype.
package detector

import (
	"encoding/binary"

	"github.com/h2non/filetype"
)

// hasPrefix returns true when buf starts with the provided byte sequence.
func hasPrefix(buf []byte, prefix ...byte) bool {
	if len(buf) == 0 {
		return false
	}
	if len(buf) < len(prefix) {
		return false
	}
	for i, b := range prefix {
		if buf[i] != b {
			return false
		}
	}
	return true
}

var dllType = filetype.NewType("dll", "application/x-msdownload")

// dllMatcher matches PE headers (MZ + optional PE\0\0 at e_lfanew) used by DLL/EXE files.
func dllMatcher(buf []byte) bool {
	if len(buf) < 2 || buf[0] != 0x4D || buf[1] != 0x5A { // "MZ"
		return false
	}

	// If we have enough bytes, also validate the PE signature at the offset stored in the DOS header.
	if len(buf) >= 0x40 {
		off := int(binary.LittleEndian.Uint32(buf[0x3C:0x40]))
		if off >= 0 && off+4 <= len(buf) && buf[off] == 0x50 && buf[off+1] == 0x45 && buf[off+2] == 0x00 && buf[off+3] == 0x00 { // "PE\0\0"
			return true
		}
		return false
	}

	// Fallback when buffer is shorter than DOS header size: accept MZ-only match.
	return true
}

var jarType = filetype.NewType("jar", "application/java-archive")

// jarMatcher matches ZIP-based JAR archives.
func jarMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x50, 0x4B, 0x03, 0x04)
}

var classType = filetype.NewType("class", "application/java-vm")

// classMatcher matches Java bytecode class files (CAFEBABE).
func classMatcher(buf []byte) bool {
	return hasPrefix(buf, 0xCA, 0xFE, 0xBA, 0xBE)
}

var pycType = filetype.NewType("pyc", "application/x-python-code")

// pycMatcher matches Python compiled bytecode headers.
func pycMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x42, 0x0D, 0x0D, 0x0A)
}

var wmaType = filetype.NewType("wma", "audio/x-ms-wma")

// wmaMatcher matches ASF/WMA GUID prefix.
func wmaMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11, 0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C)
	// &&
	// buf[8] == 0xA6 && buf[9] == 0xD9 && buf[10] == 0x00 && buf[11] == 0xAA &&
	// buf[12] == 0x00 && buf[13] == 0x62 && buf[14] == 0xCE && buf[15] == 0x6C
}

var jp2000Type = filetype.NewType("jp2", "image/jpeg")

// jp2000Matcher matches JPEG 2000 signature box header.
func jp2000Matcher(buf []byte) bool {
	return hasPrefix(buf, 0x00, 0x00, 0x00, 0x0C, 0x6A, 0x50, 0x20, 0x20)
	// &&
	// buf[8] == 0x0D && buf[9] == 0x0A && buf[10] == 0x87 && buf[11] == 0x0A
}

var pcxType = filetype.NewType("pcx", "image/x-pcx")

// pcxMatcher matches ZSoft PCX header.
func pcxMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x0A, 0x05, 0x01, 0x08)
	// &&
	// buf[4] == 0x00
}

var aiType = filetype.NewType("ai", "application/postscript")

// aiMatcher matches Adobe Illustrator (PDF-based) magic.
func aiMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x25, 0x50, 0x44, 0x46, 0x2D, 0x41) && !filetype.IsMIME(buf, "application/pdf") // "%PDF-A"
	// &&
	// buf[4] == 0x2D
}

var epsType = filetype.NewType("eps", "application/postscript")

// epsMatcher matches EPS PostScript header.
func epsMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x25, 0x21, 0x50, 0x53, 0x2D) && !filetype.IsMIME(buf, "application/pdf")
	// &&
	// buf[4] == 0x2D
}

var inddType = filetype.NewType("indd", "application/x-indesign")

// inddMatcher matches Adobe InDesign binary signature.
func inddMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x06, 0x06, 0xED, 0xF5, 0xD8, 0x1D, 0x46, 0xE5)
}

var prprojType = filetype.NewType("prproj", "application/x-prproj")

// prprojMatcher matches Adobe Premiere Pro project marker.
func prprojMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x50, 0x52, 0x50, 0xA4)
}

var xcfType = filetype.NewType("xcf", "image/x-xcf")

// xcfMatcher matches GIMP XCF signature.
func xcfMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x67, 0x69, 0x6D, 0x70, 0x20, 0x78, 0x63, 0x66)
	// &&
	// buf[8] == 0x01 && buf[9] == 0x00 && buf[10] == 0x00 && buf[11] == 0x00 &&
	// buf[12] == 0x00 && buf[13] == 0x00
}

var fbxType = filetype.NewType("fbx", "application/x-fbx")

// fbxMatcher matches Autodesk FBX magic (Kaydara binary header).
func fbxMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x4B, 0x61, 0x79, 0x64, 0x61, 0x72, 0x61, 0x58, 0x20, 0x37, 0x30, 0x30, 0x30, 0x30, 0x30)
	// && buf[7] == 0x58 &&
	// buf[8] == 0x20 && buf[9] == 0x37 && buf[10] == 0x30 && buf[11] == 0x30 &&
	// buf[12] == 0x30 && buf[13] == 0x30 && buf[14] == 0x00
	// &&
	// buf[15] == 0x1A && buf[16] == 0x00 && buf[17] == 0x00 && buf[18] == 0x00 &&
	// buf[19] == 0x00 && buf[20] == 0x00
}

var flaType = filetype.NewType("fla", "application/x-fla")

// flaMatcher matches ZIP-based Adobe FLA archives.
func flaMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x50, 0x4B, 0x03, 0x04)
}

var swcType = filetype.NewType("swc", "application/x-swc")

// swcMatcher matches ZIP-based SWC archives.
func swcMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x50, 0x4B, 0x03, 0x04)
}

var mdbType = filetype.NewType("mdb", "application/x-msaccess")

// mdbMatcher matches legacy Access MDB signature.
func mdbMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x00, 0x01, 0x00, 0x00)
	// &&
	// buf[4] == 0x53 && buf[5] == 0x74 && buf[6] == 0x61 && buf[7] == 0x6E
}

var accdbType = filetype.NewType("accdb", "application/x-msaccess-accdb")

// accdbMatcher matches ZIP-based Access ACCDB files.
func accdbMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x50, 0x4B, 0x03, 0x04)
}

var pfxType = filetype.NewType("pfx", "application/x-pkcs12")

// pfxMatcher matches PKCS#12 containers (PFX/P12).
func pfxMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x30, 0x82)
}

var dsaType = filetype.NewType("dsa", "application/x-dsa")

// dsaMatcher matches ASN.1 SEQUENCE headers used by some DSA key blobs.
func dsaMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x30 && (buf[1] == 0x81 || buf[1] == 0x82)
}

var aniType = filetype.NewType("ani", "application/x-ani")

// aniMatcher matches RIFF-based Windows cursor/animation files.
func aniMatcher(buf []byte) bool {
	return len(buf) > 12 &&
		buf[0] == 0x52 && buf[1] == 0x49 && buf[2] == 0x46 && buf[3] == 0x46 && // RIFF
		buf[8] == 0x41 && buf[9] == 0x43 && buf[10] == 0x4F && buf[11] == 0x4E // "ACON"
}

var lnkType = filetype.NewType("lnk", "application/x-ms-shortcut")

// lnkMatcher matches Windows Shell Link header and CLSID.
func lnkMatcher(buf []byte) bool {
	return len(buf) > 19 &&
		buf[0] == 0x4C && buf[1] == 0x00 && buf[2] == 0x00 && buf[3] == 0x00 && // header size 0x4C
		buf[4] == 0x01 && buf[5] == 0x14 && buf[6] == 0x02 && buf[7] == 0x00 &&
		buf[8] == 0x00 && buf[9] == 0x00 && buf[10] == 0x00 && buf[11] == 0x00 &&
		buf[12] == 0xC0 && buf[13] == 0x00 && buf[14] == 0x00 && buf[15] == 0x00 &&
		buf[16] == 0x00 && buf[17] == 0x00 && buf[18] == 0x00 && buf[19] == 0x46
}

var htmlType = filetype.NewType("html", "text/html")

// htmlMatcher matches HTML doctype prefix.
func htmlMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x3C, 0x21, 0x44, 0x4F, 0x43, 0x54, 0x59, 0x50)
}

var shType = filetype.NewType("sh", "application/x-sh")

// shMatcher matches Unix shebang for shell scripts.
func shMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x23, 0x21, 0x2F, 0x75, 0x73, 0x72, 0x2F, 0x62, 0x69, 0x6E, 0x2F, 0x73, 0x68) // "#!/usr/bin/sh"
}

var bashType = filetype.NewType("bash", "application/x-bash")

// bashMatcher matches Unix shebang for bash scripts.
func bashMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x23, 0x21, 0x2F, 0x62, 0x69, 0x6E, 0x2F, 0x62, 0x61, 0x73, 0x68) // "#!/bin/bash"
}

var plistType = filetype.NewType("plist", "application/x-plist")

// plistMatcher matches XML or binary property list headers.
func plistMatcher(buf []byte) bool {
	if hasPrefix(buf, 0x3C, 0x3F, 0x70, 0x6C, 0x69, 0x73) { // "<?plis"
		return true
	}
	if hasPrefix(buf, 0x62, 0x70, 0x6C, 0x69, 0x73, 0x74) { // "bplist"
		return true
	}
	return false
}

var torrentType = filetype.NewType("torrent", "application/x-bittorrent")

// torrentMatcher matches bencoded torrent prefix.
func torrentMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x64, 0x38, 0x3A, 0x61, 0x6E, 0x6E, 0x6F, 0x75, 0x6E, 0x63, 0x65) // "d8:announce"
}

var hdrType = filetype.NewType("hdr", "image/x-hdr")

// hdrMatcher matches Radiance HDR header.
func hdrMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x23, 0x3F, 0x52, 0x41, 0x44, 0x49, 0x41, 0x4E, 0x43, 0x45) // "#?RAD"
}

var nefType = filetype.NewType("nef", "image/x-raw-nef")

// nefMatcher matches Nikon NEF (TIFF little-endian) prefix.
func nefMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x49, 0x49, 0x2A, 0x00) // "II*."
}

var ds_storeType = filetype.NewType("ds_store", "application/x-dsstore")

// ds_storeMatcher matches Apple .DS_Store header.
func ds_storeMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x00, 0x00, 0x00, 0x01, 0x42, 0x75, 0x64, 0x31) // "\x00\x00\x00\x01bplis"
}

var icmType = filetype.NewType("icm", "application/x-iccprofile")

// icmMatcher matches ICC color profile header.
func icmMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x48, 0x43, 0x4D, 0x53) // "\x00\x00\x02\x00"
}

var phpType = filetype.NewType("php", "text/text")

// phpMatcher matches PHP script files.
func phpMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x3C, 0x3F, 0x70, 0x68, 0x70) // "<?php"
}

var m3uType = filetype.NewType("m3u", "audio/x-mpegurl")

// m3uMatcher matches M3U playlist files.
func m3uMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x23, 0x45, 0x58, 0x54, 0x4D, 0x33, 0x55) // "#EXTM3U"
}

var auType = filetype.NewType("au", "audio/basic")

// auMatcher matches Sun/NeXT AU audio file header.
func auMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x2E, 0x73, 0x6E, 0x64) // ".snd"
}

var perlType = filetype.NewType("pl", "application/x-perl")

// perlMatcher matches Perl script shebang.
func perlMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x23, 0x21, 0x2F, 0x75, 0x73, 0x72, 0x2F, 0x62, 0x69, 0x6E, 0x2F, 0x70, 0x65, 0x72, 0x6C) // "#!/usr/bin/perl"
}

var wvType = filetype.NewType("wv", "audio/x-wavpack")

// wvMatcher matches WavPack audio file header.
func wvMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x77, 0x76, 0x70, 0x6B) // "wvpk"
}

var blendType = filetype.NewType("blend", "application/x-blender")

// blendMatcher matches Blender .blend file header.
func blendMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x42, 0x4C, 0x45, 0x4E, 0x44, 0x45, 0x52) // "BLENDER"
}

var cdrType = filetype.NewType("cdr", "application/x-coreldraw")

// cdrMatcher matches CorelDRAW .cdr file header.
func cdrMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x52, 0x49, 0x46, 0x46) && // "RIFF"
		buf[8] == 0x43 && buf[9] == 0x44 && buf[10] == 0x52 && buf[11] == 0x44 // "CDRD"
}

var oneType = filetype.NewType("one", "application/x-onenote")

// oneMatcher matches Microsoft OneNote .one file header.
func oneMatcher(buf []byte) bool {
	return hasPrefix(buf, 0xE4, 0x52, 0x5C, 0x7B, 0x8C, 0xD8, 0xA7, 0x4D, 0xAE, 0xB2, 0x37, 0xA9, 0x12, 0x56, 0x8E, 0x98) // "\x0EContent"
}

var mboxType = filetype.NewType("mbox", "application/x-mbox")

// mboxMatcher matches MBOX email file header.
func mboxMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x46, 0x72, 0x6F, 0x6D, 0x20, 0x20, 0x20, 0x20) // "From    "
}

var vcfType = filetype.NewType("vcf", "text/vcard")

// vcfMatcher matches vCard file header.
func vcfMatcher(buf []byte) bool {
	return hasPrefix(buf, 0x42, 0x45, 0x47, 0x49, 0x4E, 0x3A, 0x56, 0x43, 0x41, 0x52, 0x44) // "BEGIN:VCARD"
}

// RegisterCustomMatchers wires all custom signature matchers into filetype.
func RegisterCustomMatchers() {
	// Exemple d'ajout d'un matcher personnalisé pour les fichiers ".custom"
	filetype.AddMatcher(dllType, dllMatcher)
	filetype.AddMatcher(jarType, jarMatcher)
	filetype.AddMatcher(classType, classMatcher)
	filetype.AddMatcher(pycType, pycMatcher)
	filetype.AddMatcher(wmaType, wmaMatcher)
	filetype.AddMatcher(jp2000Type, jp2000Matcher)
	filetype.AddMatcher(pcxType, pcxMatcher)
	filetype.AddMatcher(aiType, aiMatcher)
	filetype.AddMatcher(epsType, epsMatcher)
	filetype.AddMatcher(inddType, inddMatcher)
	filetype.AddMatcher(prprojType, prprojMatcher)
	filetype.AddMatcher(xcfType, xcfMatcher)
	filetype.AddMatcher(fbxType, fbxMatcher)
	filetype.AddMatcher(flaType, flaMatcher)
	filetype.AddMatcher(swcType, swcMatcher)
	filetype.AddMatcher(mdbType, mdbMatcher)
	filetype.AddMatcher(accdbType, accdbMatcher)
	filetype.AddMatcher(pfxType, pfxMatcher)
	filetype.AddMatcher(dsaType, dsaMatcher)
	filetype.AddMatcher(aniType, aniMatcher)
	filetype.AddMatcher(lnkType, lnkMatcher)
	filetype.AddMatcher(htmlType, htmlMatcher)
	filetype.AddMatcher(shType, shMatcher)
	filetype.AddMatcher(plistType, plistMatcher)
	filetype.AddMatcher(torrentType, torrentMatcher)
	filetype.AddMatcher(hdrType, hdrMatcher)
	filetype.AddMatcher(nefType, nefMatcher)
	filetype.AddMatcher(ds_storeType, ds_storeMatcher)
	filetype.AddMatcher(icmType, icmMatcher)
	filetype.AddMatcher(phpType, phpMatcher)
	filetype.AddMatcher(m3uType, m3uMatcher)
	filetype.AddMatcher(auType, auMatcher)
	filetype.AddMatcher(perlType, perlMatcher)
	filetype.AddMatcher(bashType, bashMatcher)
	filetype.AddMatcher(wvType, wvMatcher)
	filetype.AddMatcher(blendType, blendMatcher)
	filetype.AddMatcher(cdrType, cdrMatcher)
	filetype.AddMatcher(oneType, oneMatcher)
	filetype.AddMatcher(mboxType, mboxMatcher)
	filetype.AddMatcher(vcfType, vcfMatcher)
}
