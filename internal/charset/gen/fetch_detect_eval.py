#!/usr/bin/env python3
"""Write the set TestDetectEval reads: Wikipedia articles in 30 languages,
each encoded in the code pages its language is written in.

    python3 fetch_detect_eval.py DIR

The articles are fetched with the MediaWiki API (plain-text extracts),
slowly, and are not kept in the repository (CC BY-SA). A file is named
LANG.CODEC; TestDetectEval knows what each should be recognised as.
"""
import json, os, sys, time, urllib.error, urllib.parse, urllib.request

TITLES = {
    "ja": ["水", "数学"], "zh": ["水", "数学"], "zh-tw": ["水", "巴黎"], "ko": ["물", "수학"],
    "ru": ["Вода", "Математика"], "uk": ["Вода", "Математика"], "bg": ["Вода", "Математика"],
    "sr": ["Вода", "Математика"], "el": ["Νερό", "Μαθηματικά"], "pl": ["Woda", "Matematyka"],
    "cs": ["Voda", "Matematika"], "sk": ["Voda", "Matematika"], "hu": ["Víz", "Matematika"],
    "ro": ["Apă", "Matematică"], "fr": ["Eau", "Mathématiques"], "de": ["Wasser", "Mathematik"],
    "pt": ["Água", "Matemática"], "es": ["Agua", "Matemáticas"], "it": ["Acqua", "Matematica"],
    "nl": ["Water", "Wiskunde"], "sv": ["Vatten", "Matematik"], "da": ["Vand", "Matematik"],
    "fi": ["Vesi", "Matematiikka"], "ca": ["Aigua", "Matemàtiques"], "tr": ["Su", "Matematik"],
    "vi": ["Nước", "Toán học"], "he": ["מים", "מתמטיקה"], "ar": ["ماء", "رياضيات"],
    "th": ["น้ำ", "คณิตศาสตร์"], "en": ["Water", "Mathematics"],
}
CODECS = {
    "ja": ["cp932", "euc_jp", "iso2022_jp"], "zh": ["gbk", "gb18030"], "zh-tw": ["cp950"], "ko": ["cp949"],
    "ru": ["cp1251", "koi8_r", "cp866", "iso8859_5"], "uk": ["cp1251", "koi8_u"], "bg": ["cp1251"], "sr": ["cp1251"],
    "el": ["cp1253", "iso8859_7"], "pl": ["cp1250", "iso8859_2"], "cs": ["cp1250", "iso8859_2"], "sk": ["cp1250"],
    "hu": ["cp1250"], "ro": ["cp1250"], "tr": ["cp1254"], "vi": ["cp1258"], "he": ["cp1255"], "ar": ["cp1256"],
    "th": ["cp874"],
}
WESTERN = ["fr", "de", "pt", "es", "it", "nl", "sv", "da", "fi", "ca", "en"]
UTF16 = ["ru", "fr", "ja", "zh", "el"]
UTF8BAD = ["ru", "fr", "ja", "zh", "ko", "de", "el"]
UA = {"User-Agent": "go-fast-html encoding detection test (fetches a few articles once)"}


def get(url):
    for attempt in range(6):
        try:
            return json.load(urllib.request.urlopen(urllib.request.Request(url, headers=UA), timeout=30))
        except urllib.error.HTTPError as e:
            if e.code != 429:
                raise
            time.sleep(15 * (attempt + 1))
    raise RuntimeError("rate limited: " + url)


def fetch(lang):
    wiki = "zh" if lang == "zh-tw" else lang
    out = []
    for title in TITLES[lang]:
        p = {"action": "query", "prop": "extracts", "explaintext": 1, "titles": title, "format": "json"}
        if lang.startswith("zh"):
            p["variant"] = "zh-tw" if lang == "zh-tw" else "zh-cn"
        q = get(f"https://{wiki}.wikipedia.org/w/api.php?" + urllib.parse.urlencode(p))
        out += [page.get("extract", "") for page in q["query"]["pages"].values()]
        time.sleep(3)
    return "\n\n".join(out)


def main():
    d = sys.argv[1]
    os.makedirs(d, exist_ok=True)
    for lang in TITLES:
        text = fetch(lang)
        codecs = CODECS.get(lang, []) + (["cp1252"] if lang in WESTERN else []) + \
            (["utf_16_le", "utf_16_be"] if lang in UTF16 else [])
        for c in codecs:
            open(os.path.join(d, f"{lang}.{c}"), "wb").write(text.encode(c, errors="replace"))
        if lang in UTF8BAD:
            b = bytearray(text.encode("utf-8"))
            for i in range(1, len(b), 3000):
                b[i] = 0xFF
            open(os.path.join(d, f"{lang}.utf8bad"), "wb").write(bytes(b))
        print(lang, len(text), flush=True)


main()
