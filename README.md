Supported arguments
-------------------

rsync \
  -rtKOzi \
  --exclude .* \
  --exclude repodata.old \
  -e 'ssh -l sshacs -i /etc/httpd/id_rsa -o "StrictHostKeyChecking no" -o "UserKnownHostsFile /dev/null"' \
  --delete \
  --links \
  /mnt/cdn/pulp-prod-2.7/published/yum/master/yum_distributor/ubi-8-for-x86_64-appstream-source-rpms__8/1590628099.23/repodata/ \
 
 sshacs@rsync.upload.rcmnew.akadns.net:/67570/rcm/content/public/ubi/dist/ubi8/8/x86_64/appstream/source/SRPMS/repodata/


Limitations
-----------

- Only supports publishing from a local source to a remote destination
  (while rsync supports both directions).

- Will not traverse symlinks to directories.

