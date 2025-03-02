# quiki

[quiki](https://quiki.rlygd.net) is a wiki suite and standalone web server that is
completely file-based. instead of storing content in a database, each page is
represented by a text file written in the clean and productive
[quiki source language](doc/language.md) or [markdown](doc/markdown.md).

it sports caching, image generation, category management, [templates](doc/models.md),
git-based revision tracking, and more. while it is meant to be easily maintainable
from the command line, you may optionally enable the web-based editor.

* [install](#install)
* [configure](#configure)
* [run](#run)

## install

```sh
go install github.com/cooper/quiki@latest
```

## run

the easiest way to get started is run the setup wizard:

```sh
quiki -w        # or $GOPATH/bin/quiki
```

for all options see [RUNNING](RUNNING.md).