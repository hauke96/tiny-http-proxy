#! /usr/bin/env bash
#set -e

if [ $# -ne 2 ]; then
  echo "need the version number and release comment as argument"
  echo "e.g. ${0} 0.4.5 'fix local modules and modules with install_path purging bug #80 #82'"
  echo "Aborting..."
	exit 1
fi

time go test -v

if [ $? -ne 0 ]; then 
  echo "Tests unsuccessfull"
  echo "Aborting..."
	exit 1
fi

# try to get the project name from the current working directory
projectname=${PWD##*/}

echo "creating git tag v${1}"
git tag v${1}
echo "pushing git tag v${1}"
git push -f --tags
git push

test -z ${GITHUB_TOKEN} || echo "creating github release v${1}"
test -z ${GITHUB_TOKEN} && echo "skipping github-release as GITHUB_TOKEN is not set" || github-release release  --user xorpaul --repo ${projectname} --tag v${1} --name "v${1}" --description "${2}"

upx=`which upx`

### MACOS ###
echo "building and uploading ${projectname}-darwin-amd64"
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && env GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.buildtime=$BUILDTIME_v${1} -X main.buildversion=${BUILDVERSION}" && date
if [ ${#upx} -gt 0 ]; then
  $upx ${projectname}
fi
zip ${projectname}-v${1}-darwin-amd64.zip ${projectname}
test -z ${GITHUB_TOKEN} && echo "skipping github-release as GITHUB_TOKEN is not set" || github-release upload --user xorpaul --repo ${projectname} --tag v${1} --name "${projectname}-v${1}-darwin-amd64.zip" --file ${projectname}-v${1}-darwin-amd64.zip

zip ${projectname}-v${1}-darwin-amd64.zip ${projectname}
github-release upload --user xorpaul --repo ${projectname} --tag v${1} --name "${projectname}-v${1}-darwin-amd64.zip" --file ${projectname}-v${1}-darwin-amd64.zip

### LINUX ###
echo "building and uploading ${projectname}-linux-amd64"
BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S') BUILDVERSION=$(git describe --tags) && go build -race -ldflags "-X main.buildtime=$BUILDTIME_v${1} -X main.buildversion=${BUILDVERSION}" && date 
test -e /etc/os-release && ./${projectname} --version
if [ ${#upx} -gt 0 ]; then
  $upx ${projectname}
fi
zip ${projectname}-v${1}-linux-amd64.zip ${projectname}
test -z ${GITHUB_TOKEN} && echo "skipping github-release as GITHUB_TOKEN is not set" || github-release upload --user xorpaul --repo ${projectname} --tag v${1} --name "${projectname}-v${1}-linux-amd64.zip" --file ${projectname}-v${1}-linux-amd64.zip
