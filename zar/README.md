Note: Translated from the original provided TECHNICS.TXT provided with the archiver.

# Format of 'Zip Archive' files

This file contains information about the format of files that have been compressed with 
'Zip Archive'. It is primarily intended for technically minded users with some programming
knowledge.

## General

'Zip Archive' uses a slightly modified Huffman coding. This means that representative bit
sequences of variable length are generated for characters and strings. Characters or
strings that occur particularly frequently are assigned a very short bit code.

On the other hand, a relatively long code corresponds to a rare character.

As an example, consider the following text:

'CCACBBCD'

Normally, 8 bytes (8 characters ... 1 byte) of memory are used for this.
Huffman coding creates a so-called tree for generating the individual codes:

 Þ 0Þ                          Þ 1Þ
         ÚÄÄÄÄÄÄÁÄÄÄÄÄÄ¿
         C          ÚÄÄÁÄÄÄ¿
                    B    ÚÄÁÄ¿
                         D   A

This tree now assigns a code to each character A, B, C, and D. The procedure is
as follows: The code is initially empty, i.e., its length is zero. To obtain the
code for any character
x î M (M:=[A,B,C,D]), the tree is traversed from top to bottom. At each branch 
to the left, a zero is added to the code, and at each branch to the right, a one
is added. For the characters x î M, we thus obtain the bit sequences

A: 111
B: 10
C: 0
D: 110

The text 'CCACBBCD' is Compressed:

0 0 111 0 10 10 0 110

and in this form no longer occupies 2 bytes of storage space!

In 'Zip Archive' I used the highly praised 12/13-bit Huffman coding.

This specific form of the Huffman algorithm only creates codewords
with a length of 9 to 12 or 13 bits. Experiments show that the method
works most efficiently for this size. Of course, in addition to the
degree of compression, speed also plays a significant role.

It is obvious that for every data set that has been translated or
compressed with a different "lexicon" (coding tree), the lexicon must
also be saved!

## The info block

At the end of every archive created with 'Zip Archive' is the table of
contents followed by the info block. Therefore, if an archive has been saved
across multiple disks, the table of contents and the info block can be found
on the last disk.

The info block is a 7-byte data structure located at the very end of each
archive.

### It has the following format:

Bytes n-6 and n-5: Configuration (Word)
Bytes n-4 and n-3: Size of the table of contents (Word)
Bytes n-2 and n-1: Zip archive identifier 'PT'
Byte n: Version (Byte)

n denotes the size of the archive in bytes. The version indicates which version
of 'Zip Archive' was used to create the corresponding file. The four high-order
bits indicate the pre-decimal place of the version number, and the four low-order
bits indicate the decimal place of the version number.

The individual bits of the configuration word have the following meaning:

15 14 13 12 11 10 9 8 7 6 5 4 3 2 1 0
³ ³ ÀÄÄÁÄ Speed
³ ÀÄ Include directories
³
ÀÄÄ Archive contains multiple files
(number as suffix)

15 14 13 12 11 10  9  8  7  6  5  4  3  2  1  0
                         ³              ³  ÀÄÄÁÄ Speed
                         ³              ÀÄ Include directories
                         ³
                         ÀÄÄ Archive contains multiple files
                             (number as suffix)

## The table of contents

The table of contents is stored directly before the info block; its
length in bytes can be seen from the info block.

The structure of the table of contents differs slightly
when compressed with the -v option or without the -v option.

If the files are compressed with directories (-v option), the following occurs:
proceeded as follows:

### Step 1

First, ZIP.EXE stores the ASCII character #0, then the directory from which the
files are read. The directory is stored with a length byte at the beginning and
a backslash '\' at the end.

This entry could look like this:

#0#18'C:\WINDOWS\SYSTEM\'

### Step 2

Regardless of whether the files are compressed with or without directories,
'Zip-Archive' continues by saving the file attributes and the file name:

The four least significant bits of the first byte store the length of the following
file name. The remaining four bits specify the program's file attributes:

Bit 4: Read only
Bit 5: Hidden
Bit 6: System file
Bit 7: Archive

#### An example

#135'ZIP.DOK'

The first character with ASCII code 135d corresponds to the binary 1000 0111.
The length of the following string 'ZIP.DOK' is therefore 0111b=7d (the letters
b and d stand for binary and decimal) and only the archive bit is enabled.

The size of the compressed file is then stored in four consecutive bytes.

If the files are compressed without the -v option, step 2 is repeated for all
compressed files.

However, if the files are compressed with the -v option, steps 1 and 2 are repeated
for all compressed files, with step 1 being slightly modified:

Instead of the ASCII code zero, this byte stores the number of characters of the
current path that match the previously saved path. And instead of the entire
directory, only the new part, i.e., no longer matches the previously saved path,
is saved.

For example, if the three files

C:\WINDOWS\WIN.COM
C:\WINDOWS\WINDOWS.HLP
C:\WRITER\HUNZIKER\BEWERBUN.WRI

are compressed without the -v option, the following table of contents results:

#7'WIN.COM'....#11'WINDOWS.HLP'....#12'BEWERBUN.WRI'....

This It is tacitly assumed that all three files have no file attributes! The 
four dots represent the file size.

If the -v option is used during compression, the following result is obtained:

#0#11'C:\WINDOWS\'#7'WIN.COM'....#11#0'WINDOWS.HLP'....
#4#6'RITER\'#12'BEWERBUN.WRI'....

## Final note
The ZIP.EXE program was written largely in assembly language and Pascal.
Developer tools such as debuggers and profilers were also used.