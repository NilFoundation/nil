#!/bin/bash

if [ -n "$1" ]; then
    case "$1" in
    -n)
        CMD="nix develop .#update_public_repo -c "
        shift
        ;;
    *)
        echo "Unexpected option $1, for nix develop use -n."
        exit 1
        ;;
    esac
fi

CUR_REPO=$(git remote get-url origin)
if [ $? -ne 0 ]; then
    echo "Failed to get the repository URL."
    exit 1
fi

if [ -z "$CMD" ] && ! git filter-repo --help >/dev/null 2>&1; then
    echo "Error: git filter-repo is not installed locally."
    echo "Please install git filter-repo before running this script or use -n for nix develop."
    exit 1
fi

# Function to filter and push each repository
process_repo() {
    # list of paths inside main nil repo to filter commits
    COMMIT_DIRS=$1
    # that's a url of repo to which we wanna mirror to
    TARGET_URL="git@github.com:NilFoundation/$2.git"

    WORK_DIR=$(mktemp -d)
    CUR_DIR=$(pwd)
    args=""
    cur_dir=""
    for dir in $COMMIT_DIRS; do
        args+=" --path $dir"
        #cur dir is last in the list
        cur_dir=$dir
    done
    args+=" --path-rename $cur_dir/:"
    $CMD bash -c "git clone $CUR_REPO $WORK_DIR/nil && cd $WORK_DIR/nil && git filter-repo $args"
    cd $WORK_DIR/nil
    git remote add target "$TARGET_URL"
    git push target main
    cd $CUR_DIR
    rm -rf "$WORK_DIR"
}

projects=(
    # just arbitrary project names for logging
    "niljs"
    "hh-example"
    "hh-plugin"
    "uniswap"
)
project_folders=(
    # list of space separated project folders [old and current] to filter commits
    # CUR DIR IS LAST IN THE LIST
    "niljs"
    "hardhat-examples create-nil-hardhat-project"
    "hardhat-plugin"
    "uniswap"
)
pub_repos=(
    # public repos to update, github.com/NilFoundation/*.git
    "nil.js"
    "nil-hardhat-example"
    "nil-hardhat-plugin"
    "uniswap-v2-nil"
)

# Loop through each repository
for i in "${!projects[@]}"; do
    project=${projects[$i]}
    folders=${project_folders[$i]}
    repo=${pub_repos[$i]}

    echo "Processing project \`$project\` : mirroring folders [$folders] -> $repo"

    # Call the function to process the repository
    process_repo "$folders" "$repo"

    # Echo completion message
    echo "Finished processing project \`$project\`"
done
