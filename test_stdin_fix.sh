#!/bin/bash

# Test script for stdin validation fix

echo "=== MySQL Backup Helper - Stdin Validation Fix Test ==="
echo

# Test 1: Test with a file (should validate)
echo "Test 1: File validation (should validate XBSTCK01)"
printf 'XBSTCK01' > test_file.xb
echo "Command: ./mysql-backup-helper --existed-backup test_file.xb --mode=oss"
./mysql-backup-helper --existed-backup test_file.xb --mode=oss
rm -f test_file.xb
echo

# Test 2: Test with stdin (should skip validation)
echo "Test 2: Stdin validation (should skip validation)"
printf 'XBSTCK01' | ./mysql-backup-helper --existed-backup - --mode=oss
echo

# Test 3: Test with wrong file magic
echo "Test 3: Wrong file magic (should fail validation)"
printf 'WRONG01' > wrong_file.xb
echo "Command: ./mysql-backup-helper --existed-backup wrong_file.xb --mode=oss"
./mysql-backup-helper --existed-backup wrong_file.xb --mode=oss
rm -f wrong_file.xb
echo

# Test 4: Test with English language
echo "Test 4: English language test"
printf 'XBSTCK01' | ./mysql-backup-helper --existed-backup - --mode=oss --lang=en
echo

# Test 5: Test with Chinese language
echo "Test 5: Chinese language test"
printf 'XBSTCK01' | ./mysql-backup-helper --existed-backup - --mode=oss --lang=zh
echo

echo "=== Test completed ==="
