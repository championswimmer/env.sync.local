# Development

## Debian/Ubuntu build prerequisites (GUI)

The core CLI build (`make build`) does not require these GTK/WebKit dependencies.

To build the optional env-sync desktop GUI from source on Debian/Ubuntu Linux, install these system packages first:

```bash
sudo apt update
sudo apt install -y \
  build-essential \
  pkg-config \
  libgtk-3-dev \
  libwebkit2gtk-4.0-dev \
  libglib2.0-dev
```
