#!/usr/bin/env python3
"""Write testdata/detect/: short texts in many languages, each encoded in
the code pages its language is written in, for TestDetect.

The texts are written for these tests (no source). A file is named
LANG.CODEC; TestDetect knows which encoding each should be recognised as,
and which must not be decided.

    python3 gen_detect.py ../testdata/detect
"""
import os, sys, unicodedata

TEXTS = {
 "ja": "水は地球の表面の約七割を覆っており、生き物が生きていくために欠かせない物質です。私たちの体の大部分も水でできています。川や湖の水は雨や雪から生まれ、やがて海へと流れていきます。昔から人々は川の近くに町を作り、田んぼに水を引いて米を育ててきました。今日では、きれいな水を守ることが大切な課題となっています。",
 "zh": "水是地球上最常见的物质之一，覆盖了地球表面的大约百分之七十。人类的生活离不开水，农业、工业和城市都需要大量的水。中国有许多大河，例如长江和黄河，它们对国家的发展有着重要的影响。随着人口的增长，保护水资源已经成为政府和社会共同关心的问题。我们每个人都应该节约用水。",
 "zh-tw": "水是地球上最常見的物質之一，覆蓋了地球表面的大約百分之七十。人類的生活離不開水，農業、工業和城市都需要大量的水。台灣的河流大多短而急，雨季時水量很大，乾季時卻常常不足。因此政府興建了許多水庫，並且鼓勵民眾節約用水。保護水資源是我們每個人的責任。",
 "ko": "물은 지구 표면의 약 칠십 퍼센트를 덮고 있으며 모든 생명체가 살아가는 데 꼭 필요한 물질이다. 우리 몸의 대부분도 물로 이루어져 있다. 한국에는 한강과 낙동강 같은 큰 강이 있으며 예로부터 사람들은 강 근처에 마을을 만들고 농사를 지었다. 오늘날에는 깨끗한 물을 지키는 것이 중요한 과제가 되었다.",
 "ru": "Вода покрывает около семидесяти процентов поверхности Земли и необходима для всех живых существ. Человек на две трети состоит из воды. В России много больших рек, например Волга, Обь и Енисей. Люди с давних времён строили города на берегах рек, потому что вода нужна для питья, сельского хозяйства и торговли. Сегодня охрана чистой воды стала важной задачей.",
 "uk": "Вода вкриває приблизно сімдесят відсотків поверхні Землі і потрібна всім живим істотам. Людина здебільшого складається з води. В Україні є багато річок, найбільша з яких Дніпро. Люди з давніх часів будували міста на берегах річок, адже вода потрібна для пиття, сільського господарства і торгівлі. Сьогодні захист чистої води є важливим завданням.",
 "bg": "Водата покрива около седемдесет процента от повърхността на Земята и е необходима за всички живи същества. Човешкото тяло също се състои главно от вода. В България има много реки, като най-голямата е Дунав. От древни времена хората са строили градове край реките, защото водата е нужна за пиене, земеделие и търговия. Днес опазването на чистата вода е важна задача.",
 "sr": "Вода покрива око седамдесет процената површине Земље и неопходна је за сва жива бића. Људско тело се такође углавном састоји од воде. У Србији тече много река, а највеће су Дунав, Сава и Морава. Од давнина су људи градили насеља поред река, јер је вода потребна за пиће, пољопривреду и трговину. Данас је заштита чисте воде важан задатак.",
 "el": "Το νερό καλύπτει περίπου το εβδομήντα τοις εκατό της επιφάνειας της Γης και είναι απαραίτητο για όλους τους ζωντανούς οργανισμούς. Το ανθρώπινο σώμα αποτελείται κυρίως από νερό. Στην Ελλάδα υπάρχουν πολλά ποτάμια και λίμνες, αλλά το καλοκαίρι το νερό συχνά δεν αρκεί. Η προστασία του καθαρού νερού είναι σήμερα σημαντικό ζήτημα.",
 "he": "המים מכסים כשבעים אחוז משטח כדור הארץ והם חיוניים לכל היצורים החיים. גוף האדם בנוי ברובו ממים. בישראל יש מעט נהרות, והגדול שבהם הוא הירדן. מאז ומעולם בנו אנשים יישובים ליד מקורות מים, כי המים נחוצים לשתייה, לחקלאות ולמסחר. כיום שמירה על מים נקיים היא משימה חשובה.",
 "cs": "Voda pokrývá přibližně sedmdesát procent zemského povrchu a je nezbytná pro všechny živé organismy. Lidské tělo je také z velké části tvořeno vodou. Českou republikou protékají řeky Vltava, Labe a Morava. Lidé odedávna stavěli města u řek, protože voda je potřebná k pití, zemědělství a obchodu. Dnes je ochrana čisté vody důležitým úkolem.",
 "pl": "Woda pokrywa około siedemdziesięciu procent powierzchni Ziemi i jest niezbędna dla wszystkich żywych organizmów. Ciało człowieka również składa się głównie z wody. Przez Polskę płyną duże rzeki, takie jak Wisła i Odra. Od dawna ludzie budowali miasta nad rzekami, ponieważ woda jest potrzebna do picia, rolnictwa i handlu. Dziś ochrona czystej wody jest ważnym zadaniem.",
 "hu": "A víz a Föld felszínének körülbelül hetven százalékát borítja, és minden élőlény számára nélkülözhetetlen. Az emberi test is nagyrészt vízből áll. Magyarországon a legnagyobb folyók a Duna és a Tisza. Az emberek régóta folyók mellé építették városaikat, mert a víz szükséges az ivásra, a földművelésre és a kereskedelemre. Ma a tiszta víz védelme fontos feladat.",
 "tr": "Su, Dünya yüzeyinin yaklaşık yüzde yetmişini kaplar ve bütün canlılar için vazgeçilmezdir. İnsan vücudunun büyük bölümü de sudan oluşur. Türkiye'de Fırat, Dicle ve Kızılırmak gibi büyük nehirler vardır. İnsanlar eski çağlardan beri şehirlerini nehirlerin yakınına kurmuşlardır, çünkü su içmek, tarım ve ticaret için gereklidir. Bugün temiz suyu korumak önemli bir görevdir.",
 "ro": "Apa acoperă aproximativ şaptezeci la sută din suprafaţa Pământului şi este indispensabilă tuturor fiinţelor vii. Corpul omenesc este format în mare parte din apă. Prin România curg râuri mari, precum Dunărea, Mureşul şi Oltul. Oamenii au construit din vechime oraşe lângă râuri, deoarece apa este necesară pentru băut, agricultură şi comerţ. Astăzi protejarea apei curate este o sarcină importantă.",
 "vi": "Nước bao phủ khoảng bảy mươi phần trăm bề mặt Trái Đất và cần thiết cho mọi sinh vật. Cơ thể con người cũng chủ yếu được tạo thành từ nước. Việt Nam có nhiều con sông lớn như sông Hồng và sông Cửu Long. Từ xa xưa, người dân đã xây dựng làng mạc bên bờ sông vì nước cần cho việc uống, trồng lúa và buôn bán. Ngày nay bảo vệ nguồn nước sạch là một nhiệm vụ quan trọng.",
 "fr": "L'eau recouvre environ soixante-dix pour cent de la surface de la Terre et elle est indispensable à tous les êtres vivants. Le corps humain est lui aussi composé en grande partie d'eau. En France, les principaux fleuves sont la Loire, la Seine, le Rhône et la Garonne. Depuis toujours, les hommes ont bâti leurs villes près des rivières, car l'eau est nécessaire à la boisson, à l'agriculture et au commerce. Aujourd'hui, la protection de l'eau potable est un enjeu majeur.",
 "de": "Wasser bedeckt etwa siebzig Prozent der Erdoberfläche und ist für alle Lebewesen unverzichtbar. Auch der menschliche Körper besteht größtenteils aus Wasser. In Deutschland fließen große Flüsse wie der Rhein, die Elbe und die Donau. Schon früh bauten die Menschen ihre Städte an Flüssen, denn Wasser wird zum Trinken, für die Landwirtschaft und für den Handel gebraucht. Heute ist der Schutz des sauberen Wassers eine wichtige Aufgabe.",
 "es": "El agua cubre aproximadamente el setenta por ciento de la superficie de la Tierra y es imprescindible para todos los seres vivos. El cuerpo humano también está formado en gran parte por agua. En España, los ríos más largos son el Tajo, el Ebro y el Duero. Desde la antigüedad, las personas construyeron sus ciudades junto a los ríos, porque el agua es necesaria para beber, para la agricultura y para el comercio. ¿Quién no ha disfrutado de un baño en el mar?",
 "pt": "A água cobre cerca de setenta por cento da superfície da Terra e é indispensável para todos os seres vivos. O corpo humano também é formado em grande parte por água. Em Portugal, os principais rios são o Tejo, o Douro e o Guadiana. Desde a antiguidade, as populações construíram as suas cidades junto aos rios, porque a água é necessária para beber, para a agricultura e para o comércio. Hoje, a proteção da água potável é uma questão importante e as soluções não são simples.",
 "it": "L'acqua copre circa il settanta per cento della superficie terrestre ed è indispensabile per tutti gli esseri viventi. Anche il corpo umano è composto in gran parte da acqua. In Italia i fiumi più lunghi sono il Po, l'Adige e il Tevere. Fin dall'antichità gli uomini hanno costruito le loro città vicino ai fiumi, perché l'acqua serve per bere, per l'agricoltura e per il commercio. Oggi la tutela dell'acqua pulita è una questione importante, e così sarà anche in futuro.",
 "da": "Vand dækker omkring halvfjerds procent af Jordens overflade og er nødvendigt for alle levende væsener. Menneskets krop består også hovedsageligt af vand. Danmark har ingen store floder, men mange søer og en lang kyst. Gennem tiden har mennesker bygget deres byer nær vand, fordi vand bruges til drikkevand, landbrug og handel. I dag er beskyttelsen af rent grundvand en vigtig opgave for både stat og kommuner.",
}

CODECS = {
 "ja": ["cp932", "euc_jp"], "zh": ["gbk", "gb18030"], "zh-tw": ["cp950"], "ko": ["cp949"],
 "ru": ["cp1251", "koi8_r", "cp866", "iso8859_5"], "uk": ["cp1251", "koi8_u"], "bg": ["cp1251"], "sr": ["cp1251"],
 "el": ["cp1253", "iso8859_7"], "he": ["cp1255"], "cs": ["cp1250", "iso8859_2"], "pl": ["cp1250", "iso8859_2"], "hu": ["cp1250"],
 "tr": ["cp1254"], "ro": ["cp1250"], "vi": ["cp1258"],
 "fr": ["cp1252"], "de": ["cp1252"], "es": ["cp1252"], "pt": ["cp1252"], "it": ["cp1252"], "da": ["cp1252"],
}
EXTRA = {"ru": ["utf_16_le", "utf_16_be"], "fr": ["utf_16_le", "utf_16_be"]}

def encode(text, codec):
    """Encode text; for cp1258, a character it does not have is written as
    its canonical decomposition, one level at a time (ớ as ơ and U+0301),
    as Vietnamese is in that code page. A mark it has no form for is left
    out: the file is for a test that the encoding is not decided."""
    if codec != "cp1258":
        return text.encode(codec)
    def one(ch):
        try:
            return ch.encode(codec)
        except UnicodeEncodeError:
            d = unicodedata.decomposition(ch)
            return b"".join(one(chr(int(x, 16))) for x in d.split())
    return b"".join(one(ch) for ch in text)

out = sys.argv[1]
os.makedirs(out, exist_ok=True)
orig = os.path.join(os.path.dirname(os.path.abspath(out)), "text")
os.makedirs(orig, exist_ok=True)
for lang, text in TEXTS.items():
    text = "\n\n".join([text] * 3)
    # The text itself, in UTF-8, beside: what each file decodes to.
    with open(os.path.join(orig, lang + ".txt"), "w", encoding="utf-8") as f:
        f.write(text)
    for codec in CODECS[lang] + EXTRA.get(lang, []):
        with open(os.path.join(out, f"{lang}.{codec}"), "wb") as f:
            f.write(encode(text, codec))
    # UTF-8 with damaged bytes: one byte in 400 replaced by 0xFF.
    if lang in ("ja", "ru", "fr"):
        b = bytearray(text.encode("utf-8"))
        for i in range(200, len(b), 400):
            b[i] = 0xFF
        with open(os.path.join(out, f"{lang}.utf8bad"), "wb") as f:
            f.write(bytes(b))
