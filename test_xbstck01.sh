#!/bin/bash

# Test script for xbstream file validation with correct magic number

echo "=== MySQL Backup Helper - XBSTCK01 Validation Test ==="
echo

# Test 1: Test with a non-existent file
echo "Test 1: Non-existent file"
echo "Command: ./mysql-backup-helper --existed-backup /nonexistent/file.xb --mode=oss"
./mysql-backup-helper --existed-backup /nonexistent/file.xb --mode=oss
echo

# Test 2: Test with an empty file
echo "Test 2: Empty file"
touch empty_test.xb
echo "Command: ./mysql-backup-helper --existed-backup empty_test.xb --mode=oss"
./mysql-backup-helper --existed-backup empty_test.xb --mode=oss
rm -f empty_test.xb
echo

# Test 3: Test with a random binary file
echo "Test 3: Random binary file"
dd if=/dev/urandom of=random_test.xb bs=1024 count=1 2>/dev/null
echo "Command: ./mysql-backup-helper --existed-backup random_test.xb --mode=oss"
./mysql-backup-helper --existed-backup random_test.xb --mode=oss
rm -f random_test.xb
echo

# Test 4: Create a mock xbstream file with correct XBSTCK01 magic
echo "Test 4: Mock xbstream file with XBSTCK01 magic"
printf 'XBSTCK01' > mock_xbstream.xb
echo "Command: ./mysql-backup-helper --existed-backup mock_xbstream.xb --mode=oss"
./mysql-backup-helper --existed-backup mock_xbstream.xb --mode=oss
rm -f mock_xbstream.xb
echo

# Test 5: Create a mock xbstream file with wrong magic (XBST)
echo "Test 5: Mock file with wrong magic (XBST)"
printf 'XBST\x00\x00\x00\x00' > mock_wrong.xb
echo "Command: ./mysql-backup-helper --existed-backup mock_wrong.xb --mode=oss"
./mysql-backup-helper --existed-backup mock_wrong.xb --mode=oss
rm -f mock_wrong.xb
echo

# Test 6: Test with English language
echo "Test 6: English language test"
echo "Command: ./mysql-backup-helper --existed-backup /nonexistent/file.xb --mode=oss --lang=en"
./mysql-backup-helper --existed-backup /nonexistent/file.xb --mode=oss --lang=en
echo

# Test 7: Test with Chinese language
echo "Test 7: Chinese language test"
echo "Command: ./mysql-backup-helper --existed-backup /nonexistent/file.xb --mode=oss --lang=zh"
./mysql-backup-helper --existed-backup /nonexistent/file.xb --mode=oss --lang=zh
echo

echo "=== Test completed ==="
