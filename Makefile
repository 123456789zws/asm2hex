.PHONY: help clean lib build

# 检测当前操作系统
ifeq ($(OS),Windows_NT)
    PLATFORM := Windows
else
    PLATFORM := $(shell uname)
endif

# 定义变量
CAPSTONE_VERSION := 5.0.1
KEYSTONE_VERSION := 0.9.2
INSTALL_PREFIX := /usr/local
# CMAKE_POLICY_VERSION_MINIMUM keeps old Keystone/Capstone CMakeLists working on CMake 4+
CMAKE_FLAGS := -DCMAKE_INSTALL_PREFIX="$(INSTALL_PREFIX)" \
               -DCMAKE_POLICY_VERSION_MINIMUM=3.5 \
               -DCMAKE_BUILD_TYPE=Release \
               -DBUILD_SHARED_LIBS=0
MAKE_FLAGS := -j$(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)
SUDO :=

ifeq ($(PLATFORM),Windows)
    CMAKE_FLAGS += -DBUILD_LIBS_ONLY=1 \
                   -DLLVM_TARGETS_TO_BUILD="all" \
                   -G "Unix Makefiles"
    CGO_CFLAGS := -I/usr/local/include -O2 -Wall
    CGO_LDFLAGS := -L/usr/local/lib -static -lcapstone -lkeystone -lole32 -lshell32 -lkernel32 -lversion -luuid
    TARGET := windows
    KEYSTONE_BUILD_CMD := cmake $(CMAKE_FLAGS) .. && $(MAKE) $(MAKE_FLAGS) && $(MAKE) install
else ifeq ($(PLATFORM),Darwin)
    SUDO := sudo
    CGO_CFLAGS := -I/usr/local/include -O2 -Wall
    CGO_LDFLAGS := -L/usr/local/lib -lcapstone -lkeystone
    TARGET := darwin
    KEYSTONE_BUILD_CMD := cmake $(CMAKE_FLAGS) -DBUILD_LIBS_ONLY=1 .. && $(MAKE) $(MAKE_FLAGS) && $(SUDO) $(MAKE) install
else
    $(error Unsupported platform: $(PLATFORM))
endif

help:
	@echo "Available targets:"
	@echo "  help        - Show this help message"
	@echo "  clean       - Clean up temporary files"
	@echo "  lib         - Build and install Capstone and Keystone libraries"
	@echo "  build       - Build the project"
	@echo "  lib_riscv   - Build and install Capstone and Keystone libraries for RISC-V"
	@echo "  build_riscv - Build the project for RISC-V"

clean:
	@rm -rf tmp build && \
	rm -rf $(INSTALL_PREFIX)/include/capstone && \
	rm -rf $(INSTALL_PREFIX)/include/keystone && \
	rm -f $(INSTALL_PREFIX)/lib/libcapstone* && \
	rm -f $(INSTALL_PREFIX)/lib/libkeystone* && \
	go clean -cache && \
	echo "Cleaned up successfully"

lib:
	@if [ -d "$(INSTALL_PREFIX)/lib" ] && \
	   [ -f "$(INSTALL_PREFIX)/lib/libcapstone.a" ] && \
	   [ -f "$(INSTALL_PREFIX)/lib/libkeystone.a" ]; then \
		echo "Capstone and Keystone libraries are already installed"; \
	else \
		mkdir -p ./tmp && \
		cd ./tmp && \
		if [ ! -d "capstone" ]; then \
			git clone --depth 1 --branch $(CAPSTONE_VERSION) https://github.com/capstone-engine/capstone.git && \
			cd capstone && \
			mkdir build && \
			cd build && \
			cmake $(CMAKE_FLAGS) .. && \
			$(SUDO) cmake --build . --config Release --target install -- $(MAKE_FLAGS); \
		else \
			echo "Capstone library is already built, skipping build"; \
		fi && \
		cd $(CURDIR)/tmp && \
		if [ ! -d "keystone" ]; then \
			git clone --depth 1 --branch $(KEYSTONE_VERSION) https://github.com/keystone-engine/keystone.git && \
			cd keystone && \
			mkdir build && \
			cd build && \
			$(KEYSTONE_BUILD_CMD); \
		else \
			echo "Keystone library is already built, skipping build"; \
		fi && \
		echo "Libraries installed successfully for $(PLATFORM)"; \
	fi

lib_riscv:
	@if [ -d "$(INSTALL_PREFIX)/lib" ] && \
	   [ -f "$(INSTALL_PREFIX)/lib/libcapstone.a" ] && \
	   [ -f "$(INSTALL_PREFIX)/lib/libkeystone.a" ]; then \
		echo "Capstone and Keystone libraries are already installed"; \
	else \
		mkdir -p ./tmp && \
		cd ./tmp && \
		if [ ! -d "capstone" ]; then \
			git clone --depth 1 --branch $(CAPSTONE_VERSION) https://github.com/capstone-engine/capstone.git && \
			cd capstone && \
			mkdir build && \
			cd build && \
			cmake $(CMAKE_FLAGS) .. && \
			$(SUDO) cmake --build . --config Release --target install -- $(MAKE_FLAGS); \
		else \
			echo "Capstone library is already built, skipping build"; \
		fi && \
		cd $(CURDIR)/tmp && \
		if [ ! -d "keystone-riscv" ]; then \
			git clone --depth 1 --branch 0.9.3.dev2 https://github.com/null-cell/keystone.git keystone-riscv && \
			cd keystone-riscv && \
			mkdir build && \
			cd build && \
			$(KEYSTONE_BUILD_CMD); \
		else \
			echo "Keystone library is already built, skipping build"; \
		fi && \
		echo "Libraries installed successfully for $(PLATFORM)"; \
	fi

build_riscv:
	@mkdir -p ./build-riscv && \
	CGO_ENABLED=1 \
	CGO_CFLAGS="$(CGO_CFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	fyne package --release --target $(TARGET) --icon ./theme/icons/asm2hex.png  && \
	rm -rf ./build-riscv/$(if $(filter $(PLATFORM),Windows),*.exe,*.app) && \
	mv -fv $(if $(filter $(PLATFORM),Windows),*.exe,*.app) ./build-riscv && \
	echo "Build completed for $(PLATFORM)"

build:
	@mkdir -p ./build && \
	CGO_ENABLED=1 \
	CGO_CFLAGS="$(CGO_CFLAGS)" \
	CGO_LDFLAGS="$(CGO_LDFLAGS)" \
	fyne package --release --target $(TARGET) --icon ./theme/icons/asm2hex.png && \
	rm -rf ./build/$(if $(filter $(PLATFORM),Windows),*.exe,*.app) && \
	mv -fv $(if $(filter $(PLATFORM),Windows),*.exe,*.app) ./build && \
	echo "Build completed for $(PLATFORM)"
