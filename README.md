# SVARTVIT

SVARTVIT is a digital photo frame that runs on a Raspberry Pi with a Waveshare 7.5" e-Paper display (B, version 2).
It shows a random photo every hour, scaled and dithered to black and white, with today's date in the corner.

Photos are managed through a small web app where you can upload, rename, pause and delete them, preview how a photo
will look on the frame and show it right away.

## Requirements

- Go 1.27 or newer to build it
- A TrueType font for the date label (not included in the repository)
- For the frame itself: a Raspberry Pi with the Waveshare e-Paper HAT and SPI enabled (`sudo raspi-config`, then
  *Interface Options* → *SPI*)

## Building

```sh
make          # for this machine
make linux    # for the Raspberry Pi (ARMv6)
make macos    # for Intel Macs
```

## Running

```sh
./svartvit -dir ~/svartvit -font font.ttf -epd
```

Then open `http://<host>:8080` in a browser.

Leave out `-epd` to run without a display, for instance on your own computer. Each new image is then only written to
`/tmp/current.png`.

| Flag        | Default            | Description                                                    |
|-------------|--------------------|----------------------------------------------------------------|
| `-dir`      | current directory  | Where photos (`<dir>/photos`) and `database.json` are stored   |
| `-font`     | `font.ttf`         | TrueType font used for the date label                          |
| `-epd`      | off                | Show images on the e-Paper display (Raspberry Pi only)         |
| `-interval` | `1h`               | How often to change the photo, e.g. `30m`                      |
| `-overlay`  | `date`             | What to draw on top of the photo: `date` or `none`             |
| `-current`  | `/tmp/current.png` | Where to write the image currently shown on the frame          |
| `-port`     | `8080`             | Port the web app listens on                                    |

Photos can be JPEG or PNG. Set `DEVELOPMENT=true` to load the HTML templates from `templates/` instead of the ones
built into the binary, so changes show up without rebuilding.

## Using the web app

Each photo has these buttons:

- **Delete** removes the photo.
- **Pause/play** leaves the photo out of the rotation, or puts it back.
- **Edit** renames the photo.
- **Preview** opens the photo as it will look on the frame, without changing the frame.
- **Show on frame** shows the photo now. A refresh of the display takes about 30 seconds.

If you copy photos into `<dir>/photos` yourself, open `/reindex` to add them to the web app. Photos already in the
web app keep their names and paused state, and photos that have been removed from the folder are dropped.

## Deploying

`make deploy` builds for the Raspberry Pi, copies the binary to `pi@bilderamme:app/svartvit` and restarts the
`svartvit` systemd service. Change the host in the `makefile` to deploy elsewhere.

## Testing

```sh
go test ./...
```
