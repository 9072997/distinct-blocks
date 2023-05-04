# distinct-blocks
A command line utility to predict how effective deduplication (such as that available in ZFS) will be for one or more files.

## Example
Here we have a qcow2 disk image and that same disk image in flat VHDX format. We are planning on putting these on a ZFS dataset with a `recordsize` of 2mb, so we will compare these 2 files using a 2mb block size
```txt
# ls -lh
-rw-r--r--  1 scale  wheel    75G May  2 19:01 mydisk.qcow2
-rw-rw-rw-  1 root   wheel   196G May  3 03:34 mydisk.vhdx

# distinct-blocks 2m mydisk.qcow2 mydisk.vhdx
[==================] 291 GB/291 GB (0s)
total blocks: 138700, 270.9G
non-zero blocks: 61535, 150.7G
        55.63% of total
distinct blocks: 38582, 75.4G
        27.82% of total
        50.00% of non-zero
```
Here we see that we have 270.9 GB of data between the 2 files, but a lot of that is zero blocks. This makes sense since our VHDX file is flat, meaning all the thin provisioned space in the qcow2 image has been expanded, making a bunch of zeros.

We can also see that there is only 75.4 GB of unique data. If we put these files in our ZFS dataset with deduplication turned on, that is how much data we would expect to be consumed. That is about the size of our qcow2 image, which make sense because the VHDX contains the same data as the qcow2 plus a bunch of zero blocks. All the data from the qcow2 will be deduplicated with the qcow2 (leading to that "50.00% of non-zero" ratio), and all the zero blocks will be deduplicated with each other (so we are only storing a single zero block).

## Notes
You can run this on 1 or more files. It might be odd to think of deduplication on a single file, but it is possible for a file to have data duplicated within it (like the zero blocks in our example above).

This program stores the md5 sum of each distinct in a hash table in RAM. This works fine for up to a few terabytes of data, but if you have a lot of data or a very small block size, RAM can start to be an issue.

Beware of alignment issues. The whole reason I write this tool was to troubleshoot a problem where 2 almost identical files were not being deduplicated. ZFS (and many other de-duplicating filesystems) do deduplication on the block level. This means they break the file up into fixed size blocks (2mb in the example above) and check if any of them are the same. This means that ***adding or removing*** a single byte in the middle of a file will change every block after that. For example, look what happens when we replace "Here is" with "Here's"
```txt
Here is an examp    ≠    Here's an exampl
le of what it mi    ≠    e of what it mig
ght look like to    ≠    ht look like to 
 break a text fi    ≠    break a text fil
le into 16-byte     ≠    e into 16-byte b
blocks              ≠    locks
```
