# Managing release sources

`kiln fetch` is able to download BOSH releases from a few sources: AWS S3 buckets and bosh.io.

## Making Your Uncompiled Releases Fetchable for Pivotal Cloud Foundry Contributors

> Note you must have access to Pivotal's LastPass.

1. Name the release tarball so that it can be recognized by kilf fetch; it should look like this: `./$RELEASE_NAME-$RELEASE_VERSION.tgz`.

   Example: `fs-volume-2.2.3-alpha1.tgz`

1. Set S3 Credentials

   ```sh
   export AWS_ACCESS_KEY_ID="$(lpass show --notes 'pas-releng-fetch-releases' | yq -r .aws_access_key_id)"
   export AWS_SECRET_ACCESS_KEY="$(lpass show --notes 'pas-releng-fetch-releases' | yq -r .aws_secret_access_key)"
   ```

1. Upload the release

   ```sh
   aws s3 cp nfs-volume-2.2.3-alpha1.tgz s3://final-pcf-bosh-releases/
   ```

1. Update the Kiln Tile's Assets Lock

   The version for the releases you are working on should be updated in the /p-runtime/assets.yml file and the checksum member for that release must be removed (or kiln will delete the release after it is downloaded).

   ```diff
     - name: nfs-volume
   -   sha1: 81403cb99b346d2b9d6de31eeaada1de7bca3bf7
   -   version: 2.2.2
   +   version: 2.2.3-alpha1
   ```

1. (optional) To Verify Your Release is Availble

   ```sh
   kiln fetch --releases-directory releases --assets-file assets.yml --variables-file <(lpass show --notes 'pas-releng-fetch-releases')
   ```