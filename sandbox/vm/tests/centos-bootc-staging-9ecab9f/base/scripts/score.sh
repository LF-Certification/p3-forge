#!/usr/bin/env bashp

hushloginIsCorrect() {
    test -f /var/home/tux/.hushlogin &&
        test "$(stat -c '%a %u %g %s' /var/home/tux/.hushlogin)" = "644 60000 60000 0"
}

scoring::namedcheck hushlogin hushloginIsCorrect
scoring::report
