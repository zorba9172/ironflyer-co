SCHEME := IronflyerStarter
PROJECT := IronflyerStarter.xcodeproj
CONFIGURATION := Debug
DESTINATION := generic/platform=iOS Simulator

.PHONY: build generate clean install-xcodegen

build: $(PROJECT)
	xcodebuild \
		-project $(PROJECT) \
		-scheme $(SCHEME) \
		-configuration $(CONFIGURATION) \
		-sdk iphonesimulator \
		-destination '$(DESTINATION)' \
		build

$(PROJECT): xcodegen.yml
	xcodegen generate

generate:
	xcodegen generate

clean:
	rm -rf build/ DerivedData/ $(PROJECT)

install-xcodegen:
	brew install xcodegen
