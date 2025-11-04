#!/bin/bash

# Data Loss Testing Scenarios
# This script demonstrates various network failure scenarios to test data loss

set -e

echo "=================================================="
echo "PostgreSQL Data Loss Testing Scenarios"
echo "=================================================="

DB_PORT=5432
TEST_DURATION=60

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Scenario 1: Normal operation (no data loss expected)
test_normal_operation() {
    print_info "Scenario 1: Normal Operation (Baseline)"
    print_info "Running load test with no interruptions..."
    
    export TEST_RUN_DURATION=30
    export CONCURRENT_WRITERS=10
    export READ_PERCENT=50
    export INSERT_PERCENT=30
    export UPDATE_PERCENT=20
    
    go run main_v2.go
    
    print_info "Expected: 0% data loss"
    echo ""
}

# Scenario 2: Brief network interruption
test_brief_network_failure() {
    print_warning "Scenario 2: Brief Network Interruption"
    print_info "This test will temporarily block PostgreSQL connections"
    print_warning "Requires sudo access for iptables"
    
    read -p "Continue? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping..."
        return
    fi
    
    # Start load test in background
    export TEST_RUN_DURATION=$TEST_DURATION
    export CONCURRENT_WRITERS=20
    go run main_v2.go &
    TEST_PID=$!
    
    # Wait for test to start
    sleep 5
    
    # Block PostgreSQL traffic for 5 seconds
    print_warning "Blocking PostgreSQL traffic..."
    sudo iptables -A OUTPUT -p tcp --dport $DB_PORT -j DROP
    sleep 5
    
    print_info "Restoring PostgreSQL traffic..."
    sudo iptables -D OUTPUT -p tcp --dport $DB_PORT -j DROP
    
    # Wait for test to complete
    wait $TEST_PID
    
    print_info "Expected: Some data loss during network interruption"
    echo ""
}

# Scenario 3: Extended network partition
test_extended_network_partition() {
    print_warning "Scenario 3: Extended Network Partition"
    print_info "This test simulates a 15-second network partition"
    print_warning "Requires sudo access for iptables"
    
    read -p "Continue? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping..."
        return
    fi
    
    # Start load test in background
    export TEST_RUN_DURATION=$TEST_DURATION
    export CONCURRENT_WRITERS=30
    go run main_v2.go &
    TEST_PID=$!
    
    # Wait for test to start
    sleep 10
    
    # Block PostgreSQL traffic for 15 seconds
    print_warning "Creating network partition..."
    sudo iptables -A OUTPUT -p tcp --dport $DB_PORT -j DROP
    sleep 15
    
    print_info "Healing network partition..."
    sudo iptables -D OUTPUT -p tcp --dport $DB_PORT -j DROP
    
    # Wait for test to complete
    wait $TEST_PID
    
    print_info "Expected: Higher data loss due to extended partition"
    echo ""
}

# Scenario 4: Database restart during load
test_database_restart() {
    print_warning "Scenario 4: Database Restart During Load"
    print_info "This test will restart PostgreSQL during the load test"
    print_warning "Requires sudo access to restart PostgreSQL"
    
    read -p "Continue? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping..."
        return
    fi
    
    # Start load test in background
    export TEST_RUN_DURATION=$TEST_DURATION
    export CONCURRENT_WRITERS=25
    go run main_v2.go &
    TEST_PID=$!
    
    # Wait for test to start
    sleep 10
    
    # Restart PostgreSQL
    print_warning "Restarting PostgreSQL..."
    sudo systemctl restart postgresql
    
    print_info "Waiting for PostgreSQL to recover..."
    sleep 5
    
    # Wait for test to complete
    wait $TEST_PID
    
    print_info "Expected: Data loss from uncommitted transactions"
    echo ""
}

# Scenario 5: Simulated pg_rewind
test_pg_rewind_scenario() {
    print_warning "Scenario 5: Simulated pg_rewind Scenario"
    print_info "This test simulates conditions that trigger pg_rewind"
    print_info "Note: Actual pg_rewind requires HA setup with standby"
    
    read -p "Continue with simulation? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping..."
        return
    fi
    
    # High write load
    export TEST_RUN_DURATION=45
    export CONCURRENT_WRITERS=50
    export INSERT_PERCENT=70
    export UPDATE_PERCENT=30
    
    print_info "Running high write load..."
    print_warning "Manually trigger pg_rewind in another terminal if you have HA setup"
    
    go run main_v2.go
    
    print_info "In a real HA setup, pg_rewind would cause data loss"
    echo ""
}

# Scenario 6: Multiple short interruptions
test_multiple_interruptions() {
    print_warning "Scenario 6: Multiple Short Network Interruptions"
    print_info "This test creates 3 brief network interruptions"
    print_warning "Requires sudo access for iptables"
    
    read -p "Continue? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping..."
        return
    fi
    
    # Start load test in background
    export TEST_RUN_DURATION=60
    export CONCURRENT_WRITERS=20
    go run main_v2.go &
    TEST_PID=$!
    
    # Wait for test to start
    sleep 5
    
    # Create 3 interruptions
    for i in {1..3}; do
        print_warning "Interruption $i/3..."
        sudo iptables -A OUTPUT -p tcp --dport $DB_PORT -j DROP
        sleep 3
        sudo iptables -D OUTPUT -p tcp --dport $DB_PORT -j DROP
        sleep 10
    done
    
    # Wait for test to complete
    wait $TEST_PID
    
    print_info "Expected: Cumulative data loss from multiple interruptions"
    echo ""
}

# Scenario 7: High concurrency with network issue
test_high_concurrency_network_failure() {
    print_warning "Scenario 7: High Concurrency + Network Failure"
    print_info "This test uses 100 concurrent writers with network failure"
    print_warning "Requires sudo access for iptables"
    
    read -p "Continue? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping..."
        return
    fi
    
    # Start high concurrency load test
    export TEST_RUN_DURATION=45
    export CONCURRENT_WRITERS=100
    export INSERT_PERCENT=60
    export UPDATE_PERCENT=40
    go run main_v2.go &
    TEST_PID=$!
    
    # Wait for test to ramp up
    sleep 15
    
    # Brief but impactful interruption
    print_warning "Interrupting high-load operations..."
    sudo iptables -A OUTPUT -p tcp --dport $DB_PORT -j DROP
    sleep 5
    sudo iptables -D OUTPUT -p tcp --dport $DB_PORT -j DROP
    
    # Wait for test to complete
    wait $TEST_PID
    
    print_info "Expected: Significant data loss due to high concurrency"
    echo ""
}

# Main menu
show_menu() {
    echo ""
    echo "=================================================="
    echo "Select a test scenario:"
    echo "=================================================="
    echo "1. Normal Operation (Baseline - no data loss)"
    echo "2. Brief Network Interruption (5 seconds)"
    echo "3. Extended Network Partition (15 seconds)"
    echo "4. Database Restart During Load"
    echo "5. Simulated pg_rewind Scenario"
    echo "6. Multiple Short Interruptions"
    echo "7. High Concurrency + Network Failure"
    echo "8. Run All Tests (excluding DB restart)"
    echo "9. Exit"
    echo "=================================================="
    read -p "Enter choice [1-9]: " choice
    
    case $choice in
        1) test_normal_operation ;;
        2) test_brief_network_failure ;;
        3) test_extended_network_partition ;;
        4) test_database_restart ;;
        5) test_pg_rewind_scenario ;;
        6) test_multiple_interruptions ;;
        7) test_high_concurrency_network_failure ;;
        8)
            print_info "Running all non-destructive tests..."
            test_normal_operation
            test_brief_network_failure
            test_extended_network_partition
            test_multiple_interruptions
            test_high_concurrency_network_failure
            ;;
        9)
            print_info "Exiting..."
            exit 0
            ;;
        *)
            print_error "Invalid choice"
            ;;
    esac
}

# Cleanup function
cleanup() {
    print_warning "Cleaning up iptables rules..."
    sudo iptables -D OUTPUT -p tcp --dport $DB_PORT -j DROP 2>/dev/null || true
    print_info "Cleanup complete"
}

# Trap cleanup on script exit
trap cleanup EXIT

# Check if running as root for iptables tests
check_sudo() {
    if ! sudo -n true 2>/dev/null; then
        print_warning "Some tests require sudo access for iptables"
        print_warning "You may be prompted for password"
    fi
}

# Main
main() {
    print_info "PostgreSQL Data Loss Testing Framework"
    print_info "Database Port: $DB_PORT"
    
    check_sudo
    
    while true; do
        show_menu
    done
}

# Run main
main
