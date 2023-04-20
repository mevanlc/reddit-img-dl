# reddit-img-dl

Downloader for media submissions to reddit.com. Supports both subreddits and users.

Fork of [reddit-dl](https://github.com/The-Eye-Team/reddit-dl).

## Build Instructions

Download binaries from github releases or compile it yourself with:

```
mkdir -p ./bin && go build -o ./bin/reddit-img-dl .
```

## Why Fork the upstream project ?

`reddit-dl` downloads images/videos in a database like structure, with thousands of deep nested subfolders and non-sensical filenames. Its meant for archival, with metadata stored in a sqlite database. This structure is a pain in the ass if you want to browse via a file manager or gallery app/webapp and it's almost impossible to find something without consulting the metadata db.

Changes:

- Fixed and added support for some image/gif hosts.
- Downloads all files in a single folder, instead of nested folders chosen according to afew digits of the image's id.
- All images have the post title in the filename so it's easy to search them from a file manager.

If you want to archive subreddits, then I'd recommend the [upstream project.](https://github.com/The-Eye-Team/reddit-dl)

## Usage

Run in your terminal:

```
./reddit-img-dl --subreddit pics --concurrency 50
```

Options:

```sh
    --concurrency int   Maximum number of simultaneous downloads. (default 10)
    --mbpp-bar-gradient Enabling this will make the bar gradient from red/yellow/green.
    --save-dir string   Path to a directory to save to.
-r, --subreddit string  The name of a subreddit to archive. (ex. AskReddit, unixporn, CasualConversation, etc.)
-u, --user string       The name of a user to archive. (ex. spez, PoppinKREAM, Shitty_Watercolour, etc.)
```

The flags `-r` and `-u` may be passed multiple times to download many reddits at once.

## License

MIT
