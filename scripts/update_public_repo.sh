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
    TARGET_PATH=$1
    TARGET_URL=$2

    WORK_DIR=$(mktemp -d)
    CUR_DIR=$(pwd)
    bash -c "$CMD git clone $CUR_REPO $WORK_DIR/$TARGET_PATH && cd $WORK_DIR/$TARGET_PATH && git filter-repo --path $TARGET_PATH --path-rename $TARGET_PATH/:"
    cd $WORK_DIR/$TARGET_PATH
    git remote add target "$TARGET_URL"
    git push target main
    cd $CUR_DIR
    rm -rf "$WORK_DIR"
}

repos=(
    "niljs git@github.com:NilFoundation/nil.js.git"
    "hardhat-examples git@github.com:NilFoundation/nil-hardhat-example.git"
    "hardhat-plugin git@github.com:NilFoundation/nil-hardhat-plugin.git"
)

# Loop through each repository
for repo in "${repos[@]}"; do
    # Split the repo string into name and URL
    repo_name=$(echo "$repo" | awk '{print $1}')
    repo_url=$(echo "$repo" | awk '{print $2}')

    # Echo the repository being processed
    echo "Processing repository: $repo_name ($repo_url)"

    # Call the function to process the repository
    process_repo "$repo_name" "$repo_url"

    # Echo completion message
    echo "Finished processing repository: $repo_name"
done

