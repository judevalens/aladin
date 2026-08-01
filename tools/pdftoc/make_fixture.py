"""Builds a book-shaped test PDF: unnumbered front matter, a roman-numbered preface,
a printed contents page, then an arabic-numbered body. The roman and arabic runs have
DIFFERENT offsets, which is exactly where naive tools go wrong."""
from reportlab.lib.pagesizes import letter
from reportlab.pdfgen import canvas

W, H = letter
c = canvas.Canvas("sample-book.pdf", pagesize=letter)


def page(title, body, folio=None, size=20):
    c.setFont("Helvetica-Bold", size)
    c.drawString(90, H - 140, title)
    c.setFont("Helvetica", 11)
    y = H - 180
    for line in body:
        c.drawString(90, y, line)
        y -= 16
    if folio:
        c.setFont("Helvetica", 10)
        c.drawCentredString(W / 2, 50, folio)
    c.showPage()


page("The Shape of Drift", ["An imaginary book, for testing."], size=26)
page("Copyright", ["(c) nobody. Invented for a fixture."])
page("Contents", [
    "Preface . . . . . . . . . . . . . . . . . . . .  vii",
    "1  Beginnings . . . . . . . . . . . . . . . . .   1",
    "    1.1  First light . . . . . . . . . . . . . .   2",
    "    1.2  Second wind . . . . . . . . . . . . . .   3",
    "2  Middles . . . . . . . . . . . . . . . . . . .   4",
    "    2.1  The long part . . . . . . . . . . . . .   5",
    "3  Ends . . . . . . . . . . . . . . . . . . . . .  6",
], folio="v")
page("Preface", ["Why this book exists."], folio="vii")
page("1  Beginnings", ["Where drift starts."], folio="1")
page("1.1  First light", ["The earliest observation."], folio="2")
page("1.2  Second wind", ["It recurs."], folio="3")
page("2  Middles", ["The long middle."], folio="4")
page("2.1  The long part", ["Longer still."], folio="5")
page("3  Ends", ["How it resolves."], folio="6")

c.save()
print("sample-book.pdf: 10 pages")
print("  printed 1   -> PDF 5   (arabic offset 4)")
print("  printed vii -> PDF 4   (roman offset -3)")
