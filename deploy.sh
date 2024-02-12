#! /bin/bash
npm run build
ruby version.rb
GOOS=linux GOARCH=amd64 go build -o jairo.cloud main.go
ssh root@galax.be -p 1000 "pm2 stop 2"
scp -P 1000 ./jairo.cloud root@galax.be:/root
ssh root@galax.be -p 1000 "pm2 restart 2"