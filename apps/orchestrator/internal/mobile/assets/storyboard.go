package assets

import (
	"fmt"
	"html"
	"image/color"
	"strings"
)

// StoryboardParams is the data the LaunchScreen.storyboard template
// needs at fill-in time. The two pre-parsed RGBA values let us emit
// Apple's normalized (0..1) floats without re-parsing the hex string.
type StoryboardParams struct {
	AppName        string
	BackgroundHex  string
	ForegroundHex  string
	BackgroundRGBA color.RGBA
	ForegroundRGBA color.RGBA
}

// rgbaFloats turns an 8-bit color into the four normalized floats
// Apple's .storyboard XML expects.
func rgbaFloats(c color.RGBA) (string, string, string, string) {
	return fmt.Sprintf("%.6f", float64(c.R)/255.0),
		fmt.Sprintf("%.6f", float64(c.G)/255.0),
		fmt.Sprintf("%.6f", float64(c.B)/255.0),
		fmt.Sprintf("%.6f", float64(c.A)/255.0)
}

// RenderLaunchStoryboard fills the parameterised storyboard template.
// The output is byte-stable for the same parameters — used by the
// generator's determinism contract.
func RenderLaunchStoryboard(p StoryboardParams) string {
	if p.AppName == "" {
		p.AppName = "App"
	}
	bgR, bgG, bgB, bgA := rgbaFloats(p.BackgroundRGBA)
	fgR, fgG, fgB, fgA := rgbaFloats(p.ForegroundRGBA)

	out := launchStoryboardTemplate
	out = strings.ReplaceAll(out, "{{APP_NAME}}", html.EscapeString(p.AppName))
	out = strings.ReplaceAll(out, "{{BG_R}}", bgR)
	out = strings.ReplaceAll(out, "{{BG_G}}", bgG)
	out = strings.ReplaceAll(out, "{{BG_B}}", bgB)
	out = strings.ReplaceAll(out, "{{BG_A}}", bgA)
	out = strings.ReplaceAll(out, "{{FG_R}}", fgR)
	out = strings.ReplaceAll(out, "{{FG_G}}", fgG)
	out = strings.ReplaceAll(out, "{{FG_B}}", fgB)
	out = strings.ReplaceAll(out, "{{FG_A}}", fgA)
	return out
}

// launchStoryboardTemplate is the .storyboard XML Apple's Xcode parses
// for the launch screen. A centered UIImageView (driven by the
// generated AppIcon-1024) sits above a UILabel rendering the app name.
// The XML is intentionally minimal: no scenes other than the launch
// screen, no segues, no autoresizing magic that breaks at compile time.
const launchStoryboardTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.Storyboard.XIB" version="3.0" toolsVersion="22155" targetRuntime="iOS.CocoaTouch" propertyAccessControl="none" useAutolayout="YES" launchScreen="YES" useTraitCollections="YES" useSafeAreas="YES" colorMatched="YES" initialViewController="ironflyer-launch-vc">
    <device id="retina6_1" orientation="portrait" appearance="light"/>
    <dependencies>
        <plugIn identifier="com.apple.InterfaceBuilder.IBCocoaTouchPlugin" version="22131"/>
        <capability name="Safe area layout guides" minToolsVersion="9.0"/>
        <capability name="documents saved in the Xcode 8 format" minToolsVersion="8.0"/>
    </dependencies>
    <scenes>
        <scene sceneID="ironflyer-launch-scene">
            <objects>
                <viewController id="ironflyer-launch-vc" sceneMemberID="viewController">
                    <view key="view" contentMode="scaleToFill" id="ironflyer-launch-view">
                        <rect key="frame" x="0.0" y="0.0" width="390" height="844"/>
                        <autoresizingMask key="autoresizingMask" widthSizable="YES" heightSizable="YES"/>
                        <subviews>
                            <imageView clipsSubviews="YES" userInteractionEnabled="NO" contentMode="scaleAspectFit" image="LaunchLogo" translatesAutoresizingMaskIntoConstraints="NO" id="ironflyer-launch-image">
                                <rect key="frame" x="115" y="322" width="160" height="160"/>
                            </imageView>
                            <label opaque="NO" userInteractionEnabled="NO" contentMode="left" horizontalHuggingPriority="251" verticalHuggingPriority="251" text="{{APP_NAME}}" textAlignment="center" lineBreakMode="tailTruncation" baselineAdjustment="alignBaselines" adjustsFontSizeToFit="NO" translatesAutoresizingMaskIntoConstraints="NO" id="ironflyer-launch-label">
                                <rect key="frame" x="20" y="502" width="350" height="32"/>
                                <fontDescription key="fontDescription" type="boldSystem" pointSize="22"/>
                                <color key="textColor" red="{{FG_R}}" green="{{FG_G}}" blue="{{FG_B}}" alpha="{{FG_A}}" colorSpace="custom" customColorSpace="sRGB"/>
                                <nil key="highlightedColor"/>
                            </label>
                        </subviews>
                        <viewLayoutGuide key="safeArea" id="ironflyer-launch-safearea"/>
                        <color key="backgroundColor" red="{{BG_R}}" green="{{BG_G}}" blue="{{BG_B}}" alpha="{{BG_A}}" colorSpace="custom" customColorSpace="sRGB"/>
                        <constraints>
                            <constraint firstItem="ironflyer-launch-image" firstAttribute="centerX" secondItem="ironflyer-launch-view" secondAttribute="centerX" id="img-center-x"/>
                            <constraint firstItem="ironflyer-launch-image" firstAttribute="centerY" secondItem="ironflyer-launch-view" secondAttribute="centerY" constant="-40" id="img-center-y"/>
                            <constraint firstItem="ironflyer-launch-image" firstAttribute="width" constant="160" id="img-width"/>
                            <constraint firstItem="ironflyer-launch-image" firstAttribute="height" constant="160" id="img-height"/>
                            <constraint firstItem="ironflyer-launch-label" firstAttribute="centerX" secondItem="ironflyer-launch-view" secondAttribute="centerX" id="lbl-center-x"/>
                            <constraint firstItem="ironflyer-launch-label" firstAttribute="top" secondItem="ironflyer-launch-image" secondAttribute="bottom" constant="20" id="lbl-top"/>
                            <constraint firstItem="ironflyer-launch-label" firstAttribute="leading" secondItem="ironflyer-launch-view" secondAttribute="leading" constant="20" id="lbl-leading"/>
                            <constraint firstItem="ironflyer-launch-view" firstAttribute="trailing" secondItem="ironflyer-launch-label" secondAttribute="trailing" constant="20" id="lbl-trailing"/>
                        </constraints>
                    </view>
                </viewController>
                <placeholder placeholderIdentifier="IBFirstResponder" id="ironflyer-launch-responder" userLabel="First Responder" sceneMemberID="firstResponder"/>
            </objects>
            <point key="canvasLocation" x="53" y="375"/>
        </scene>
    </scenes>
    <resources>
        <image name="LaunchLogo" width="160" height="160"/>
    </resources>
</document>
`
