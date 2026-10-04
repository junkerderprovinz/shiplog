# Marks on the download buttons

The path data `../gen_download_buttons.py` draws into the README's download
buttons. `<name>.txt` holds the `d` attribute of each of the mark's paths, one
per line, and `<name>.box.txt` the viewBox it was drawn in, which is what lets
the generator scale marks of different widths to one optical size.

## Source and licence

**Font Awesome Free 6.7.2**, from <https://fontawesome.com>: the ZIP
(`file-zipper`) from the `solid` set. The icons are **CC BY 4.0**, which asks
for attribution and nothing else. Copyright 2024 Fonticons, Inc.

**Dashboard Icons** (`homarr-labs/dashboard-icons`, <https://dashboardicons.com>):
Unraid, its `unraid.svg` with the gradient left out, since the button draws it
in one ink. **Apache-2.0**, whose licence text is beside this file as
`LICENSE-dashboard-icons.txt`. Copyright the Homarr Labs team and contributors.

## Trademarks

The Unraid mark is a trademark of Lime Technology. It is used the one way a
trademark may be used without permission, which is to refer to the thing it
names: it sits on the button that installs ShipLog on Unraid, unmodified, and
nothing here claims endorsement by or affiliation with Lime Technology. The ZIP
is no one's mark; it stands for the source archive.

## Adding one

Take the SVG, keep its `viewBox` verbatim in `<name>.box.txt`, and put the `d`
attribute of each path on a line of its own in `<name>.txt`, a `rect` written as
a path. The button draws every path in one ink, so the mark's colours are lost.
