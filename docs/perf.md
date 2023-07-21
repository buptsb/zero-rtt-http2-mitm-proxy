- start chrome

google-chrome-beta --remote-debugging-port=9222 --proxy-server="http://localhost:8080" --user-data-dir=/tmp/userdir

- record har using CDP

./node_modules/chrome-har-capturer/bin/cli.js https://news.ycombinator.com -o 2.har

- har online viewer

http://www.softwareishard.com/har/viewer/