#!/bin/bash
set -euxo pipefail
c=$USER-tgrbtl
cat > install.sh <<EOF
#!/bin/bash
set -euxo pipefail
if [ -f tigerbeetle ]; then
  exit 0
fi
git clone https://github.com/tigerbeetledb/tigerbeetle.git tb-repo || true
(cd tb-repo && scripts/install.sh)
mv tb-repo/tigerbeetle tigerbeetle
touch needs-update
EOF
chmod +x install.sh
roachprod put $c install.sh
roachprod ssh $c -- ./install.sh

for i in $(seq 1 3); do
	roachprod ssh $c:$i -- "
[ -f needs-update ] && tb-repo/scripts/upgrade_ubuntu_kernel.sh && rm -f needs-update && sudo reboot" || true
done

roachprod ssh $c -- rm -rf /mnt/data1/tigerbeetle
for i in $(seq 1 3); do
	roachprod ssh $c:$i -- ./tigerbeetle format --cluster="0" --replica="$((i-1))" /mnt/data1/tigerbeetle
done

# NB: for some reason it can't bind to the local --external address :shrug:
# The sliced is indexed by replicaID... so very annoying to script.
addrs=$(roachprod ip tobias-tgrbtl | sed 's/$/:3000/g' | paste -sd ',' -)
roachprod ssh $c -- sudo systemd-run --unit tigerbeetle --same-dir ./tigerbeetle start --addresses="${addrs}" /mnt/data1/tigerbeetle
