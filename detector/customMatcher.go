package detector

import "github.com/h2non/filetype"

var dllType = filetype.NewType("dll", "application/x-msdownload")

func dllMatcher(buf []byte) bool {
	return len(buf) > 2 && buf[0] == 0x4D && buf[1] == 0x5A
}

var jarType = filetype.NewType("jar", "application/java-archive")

func jarMatcher(buf []byte) bool {
	return len(buf) > 4 && buf[0] == 0x50 && buf[1] == 0x4B && buf[2] == 0x03 && buf[3] == 0x04
}

var classType = filetype.NewType("class", "application/java-vm")

func classMatcher(buf []byte) bool {
	return len(buf) > 4 && buf[0] == 0xCA && buf[1] == 0xFE && buf[2] == 0xBA && buf[3] == 0xBE
}

var pycType = filetype.NewType("pyc", "application/x-python-code")

func pycMatcher(buf []byte) bool {
	return len(buf) > 4 && buf[0] == 0x42 && buf[1] == 0x0D && buf[2] == 0x0D && buf[3] == 0x0A
}

var wmaType = filetype.NewType("wma", "audio/x-ms-wma")

func wmaMatcher(buf []byte) bool {
	return len(buf) > 8 &&
		buf[0] == 0x30 && buf[1] == 0x26 && buf[2] == 0xB2 && buf[3] == 0x75 &&
		buf[4] == 0x8E && buf[5] == 0x66 && buf[6] == 0xCF && buf[7] == 0x11
	// &&
	// buf[8] == 0xA6 && buf[9] == 0xD9 && buf[10] == 0x00 && buf[11] == 0xAA &&
	// buf[12] == 0x00 && buf[13] == 0x62 && buf[14] == 0xCE && buf[15] == 0x6C
}

var jp2000Type = filetype.NewType("jp2", "image/jpeg")

func jp2000Matcher(buf []byte) bool {
	return len(buf) > 8 &&
		buf[0] == 0x00 && buf[1] == 0x00 && buf[2] == 0x00 && buf[3] == 0x0C &&
		buf[4] == 0x6A && buf[5] == 0x50 && buf[6] == 0x20 && buf[7] == 0x20
	// &&
	// buf[8] == 0x0D && buf[9] == 0x0A && buf[10] == 0x87 && buf[11] == 0x0A
}

var pcxType = filetype.NewType("pcx", "image/x-pcx")

func pcxMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x0A && buf[1] == 0x05 && buf[2] == 0x01 && buf[3] == 0x08
	// &&
	// buf[4] == 0x00
}

var aiType = filetype.NewType("ai", "application/postscript")

func aiMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x25 && buf[1] == 0x50 && buf[2] == 0x44 && buf[3] == 0x46
	// &&
	// buf[4] == 0x2D
}

var epsType = filetype.NewType("eps", "application/postscript")

func epsMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x25 && buf[1] == 0x21 && buf[2] == 0x50 && buf[3] == 0x53
	// &&
	// buf[4] == 0x2D
}

var inddType = filetype.NewType("indd", "application/x-indesign")

func inddMatcher(buf []byte) bool {
	return len(buf) > 8 &&
		buf[0] == 0x06 && buf[1] == 0x06 && buf[2] == 0xED && buf[3] == 0xF5 && buf[4] == 0xD8 && buf[5] == 0x1D && buf[6] == 0x46 && buf[7] == 0xE5
}

var prprojType = filetype.NewType("prproj", "application/x-prproj")

func prprojMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x50 && buf[1] == 0x52 && buf[2] == 0x50 && buf[3] == 0xA4
}

var xcfType = filetype.NewType("xcf", "image/x-xcf")

func xcfMatcher(buf []byte) bool {
	return len(buf) > 14 &&
		buf[0] == 0x67 && buf[1] == 0x69 && buf[2] == 0x6D && buf[3] == 0x70 &&
		buf[4] == 0x20 && buf[5] == 0x78 && buf[6] == 0x63 && buf[7] == 0x66
	// &&
	// buf[8] == 0x01 && buf[9] == 0x00 && buf[10] == 0x00 && buf[11] == 0x00 &&
	// buf[12] == 0x00 && buf[13] == 0x00
}

var fbxType = filetype.NewType("fbx", "application/x-fbx")

func fbxMatcher(buf []byte) bool {
	return len(buf) > 21 &&
		buf[0] == 0x4B && buf[1] == 0x61 && buf[2] == 0x79 && buf[3] == 0x64 &&
		buf[4] == 0x61 && buf[5] == 0x72 && buf[6] == 0x61
	// && buf[7] == 0x58 &&
	// buf[8] == 0x20 && buf[9] == 0x37 && buf[10] == 0x30 && buf[11] == 0x30 &&
	// buf[12] == 0x30 && buf[13] == 0x30 && buf[14] == 0x00
	// &&
	// buf[15] == 0x1A && buf[16] == 0x00 && buf[17] == 0x00 && buf[18] == 0x00 &&
	// buf[19] == 0x00 && buf[20] == 0x00
}

var flaType = filetype.NewType("fla", "application/x-fla")

func flaMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x50 && buf[1] == 0x4B && buf[2] == 0x03 && buf[3] == 0x04
}

var swcType = filetype.NewType("swc", "application/x-swc")

func swcMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x50 && buf[1] == 0x4B && buf[2] == 0x03 && buf[3] == 0x04
}

var mdbType = filetype.NewType("mdb", "application/x-msaccess")

func mdbMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x00 && buf[1] == 0x01 && buf[2] == 0x00 && buf[3] == 0x00
	// &&
	// buf[4] == 0x53 && buf[5] == 0x74 && buf[6] == 0x61 && buf[7] == 0x6E
}

var accdbType = filetype.NewType("accdb", "application/x-msaccess-accdb")

func accdbMatcher(buf []byte) bool {
	return len(buf) > 8 &&
		buf[0] == 0x50 && buf[1] == 0x4B && buf[2] == 0x03 && buf[3] == 0x04
}

var pstType = filetype.NewType("pst", "application/x-pst")

func pstMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x21 && buf[1] == 0x42 && buf[2] == 0x44 && buf[3] == 0x4E
}

var pfxType = filetype.NewType("pfx", "application/x-pkcs12")

func pfxMatcher(buf []byte) bool {
	return len(buf) > 2 &&
		buf[0] == 0x30 && buf[1] == 0x82
}

var keyNoteType = filetype.NewType("key", "application/x-keynote")

func keyNoteMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x50 && buf[1] == 0x4B && buf[2] == 0x03 && buf[3] == 0x04
}

var dsaType = filetype.NewType("dsa", "application/x-dsa")

func dsaMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x30 && buf[1] == 0x81 || buf[1] == 0x82
}

var aniType = filetype.NewType("ani", "application/x-ani")

func aniMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x52 && buf[1] == 0x49 && buf[2] == 0x46 && buf[3] == 0x46
}

var lnkType = filetype.NewType("lnk", "application/x-ms-shortcut")

func lnkMatcher(buf []byte) bool {
	return len(buf) > 4 &&
		buf[0] == 0x4C && buf[1] == 0x00 && buf[2] == 0x00 && buf[3] == 0x00
}

var xmlType = filetype.NewType("xml", "application/xml")

func xmlMatcher(buf []byte) bool {
	return len(buf) > 5 &&
		(buf[0] == 0x3C && buf[1] == 0x3F && buf[2] == 0x78 && buf[3] == 0x6D && buf[4] == 0x6C) // "<?xml"
}

var htmlType = filetype.NewType("html", "text/html")

func htmlMatcher(buf []byte) bool {
	return len(buf) > 15 &&
		(buf[0] == 0x3C && buf[1] == 0x21 && buf[2] == 0x44 && buf[3] == 0x4F &&
			buf[4] == 0x43 && buf[5] == 0x54 && buf[6] == 0x59 && buf[7] == 0x50 &&
			buf[8] == 0x45) // "<!DOCTYPE"
}

var shType = filetype.NewType("sh", "application/x-sh")

func shMatcher(buf []byte) bool {
	return len(buf) > 2 &&
		(buf[0] == 0x23 && buf[1] == 0x21 && buf[2] == 0x2F) // "#!/"
}

var plistType = filetype.NewType("plist", "application/x-plist")

func plistMatcher(buf []byte) bool {
	return len(buf) > 5 &&
		(buf[0] == 0x3C && buf[1] == 0x3F && buf[2] == 0x70 && buf[3] == 0x6C) || (buf[0] == 0x62 && buf[1] == 0x70 && buf[2] == 0x6C && buf[3] == 0x69 && buf[4] == 0x73 && buf[5] == 0x74) // "<?plis"
}

var torrentType = filetype.NewType("torrent", "application/x-bittorrent")

func torrentMatcher(buf []byte) bool {
	return len(buf) > 3 &&
		(buf[0] == 0x64 && buf[1] == 0x38 && buf[2] == 0x3A) // "d8"
}

var hdrType = filetype.NewType("hdr", "image/x-hdr")

func hdrMatcher(buf []byte) bool {
	return len(buf) > 9 &&
		(buf[0] == 0x23 && buf[1] == 0x3F && buf[2] == 0x52 && buf[3] == 0x41 && buf[4] == 0x44 && buf[5] == 0x49 && buf[6] == 0x41 && buf[7] == 0x4E && buf[8] == 0x43 && buf[9] == 0x45) // "#?RAD"
}

var nefType = filetype.NewType("nef", "image/x-raw-nef")

func nefMatcher(buf []byte) bool {
	return len(buf) > 3 &&
		(buf[0] == 0x49 && buf[1] == 0x49 && buf[2] == 0x2A && buf[3] == 0x00) // "II*."
}

var ds_storeType = filetype.NewType("ds_store", "application/x-dsstore")

func ds_storeMatcher(buf []byte) bool {
	return len(buf) > 7 &&
		// (buf[0] == 0x62 && buf[1] == 0x70 && buf[2] == 0x6C && buf[3] == 0x69 && buf[4] == 0x73) // "bplis"
		(buf[0] == 0x00 && buf[1] == 0x00 && buf[2] == 0x00 && buf[3] == 0x01 && buf[4] == 0x42 && buf[5] == 0x75 && buf[6] == 0x64 && buf[7] == 0x31) // "\x00\x00\x00\x01bplis"
}

var icmType = filetype.NewType("icm", "application/x-iccprofile")

func icmMatcher(buf []byte) bool {
	return len(buf) > 3 &&
		(buf[0] == 0x48 && buf[1] == 0x43 && buf[2] == 0x4D && buf[3] == 0x53) // "\x00\x00\x02\x00"
}

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
	filetype.AddMatcher(pstType, pstMatcher)
	filetype.AddMatcher(pfxType, pfxMatcher)
	filetype.AddMatcher(keyNoteType, keyNoteMatcher)
	filetype.AddMatcher(dsaType, dsaMatcher)
	filetype.AddMatcher(aniType, aniMatcher)
	filetype.AddMatcher(lnkType, lnkMatcher)
	filetype.AddMatcher(xmlType, xmlMatcher)
	filetype.AddMatcher(htmlType, htmlMatcher)
	filetype.AddMatcher(shType, shMatcher)
	filetype.AddMatcher(plistType, plistMatcher)
	filetype.AddMatcher(torrentType, torrentMatcher)
	filetype.AddMatcher(hdrType, hdrMatcher)
	filetype.AddMatcher(nefType, nefMatcher)
	filetype.AddMatcher(ds_storeType, ds_storeMatcher)
	filetype.AddMatcher(icmType, icmMatcher)

}
