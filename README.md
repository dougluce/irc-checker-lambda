# irc.horph.com checker for AWS Lambda

This runs a few simple checks on the IRC server to make sure it looks
like it's still function. This means connect via TLS, ensure a user is
there, and check that the reported uptime is fairly long (more than
one INTERVAL).

## To (re-)deploy

Build it:

    GOOS=linux go build check.go

Zip it:

    zip function.zip check

Ship it:

    aws lambda update-function-code \
    --function-name irc-horph-com_check \
    --zip-file fileb://function.zip
