"""The download buttons the README shows, read by gen_download_buttons.py.

Each entry names a button the generator knows and where it leads. The rows,
their order, the colours and the words are the generator's, the same in every
repository.
"""

REPO = "shiplog"

# ShipLog ships as an Unraid plugin; its container image only carries the engine
# for other Docker hosts, so there is no Docker button.
BUTTONS = {
    "unraid": "https://ca.unraid.net/apps/shiplog-1lpuit5150ztw5",
    # A release's "Source code (zip)" is the whole repository at that tag, and
    # GitHub gives the newest one no fixed address, so this leads to the release
    # that lists it.
    "source": "https://github.com/junkerderprovinz/shiplog/releases/latest",
}
