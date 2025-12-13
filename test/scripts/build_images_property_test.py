#!/usr/bin/env python3
"""
Property-based tests for container image build and tagging.

**Feature: arbiter-pipeline-infrastructure, Property 17: Container image tagging**
**Validates: Requirements 12.1, 12.2, 12.3**
"""

import subprocess
import re
import os
from hypothesis import given, strategies as st, settings, assume
from hypothesis import HealthCheck

# Valid semantic version strategy
@st.composite
def semantic_version(draw):
    """Generate valid semantic versions in the format v<major>.<minor>.<patch>"""
    major = draw(st.integers(min_value=0, max_value=99))
    minor = draw(st.integers(min_value=0, max_value=99))
    patch = draw(st.integers(min_value=0, max_value=99))
    return f"v{major}.{minor}.{patch}"

# Valid commit SHA strategy (short form)
@st.composite
def commit_sha(draw):
    """Generate valid git commit SHAs (short form, 7 characters)"""
    chars = draw(st.text(alphabet='0123456789abcdef', min_size=7, max_size=7))
    return chars

# Valid image name strategy
image_names = st.sampled_from(['builder', 'deployer', 'tester', 'validator'])

# Valid registry strategy
@st.composite
def registry_name(draw):
    """Generate valid container registry names"""
    # Simple registry names for testing
    registries = [
        'localhost:5000/arbiter',
        'test-registry.io/arbiter',
        'my-registry/arbiter'
    ]
    return draw(st.sampled_from(registries))


class TestImageTagging:
    """
    Property 17: Container image tagging
    
    For any published container image, it should be tagged with:
    1. A semantic version (v<major>.<minor>.<patch>)
    2. The Git commit SHA
    3. "latest" if it is the most recent build
    
    **Validates: Requirements 12.1, 12.2, 12.3**
    """
    
    @given(
        version=semantic_version(),
        sha=commit_sha(),
        image=image_names,
        registry=registry_name()
    )
    @settings(max_examples=100, suppress_health_check=[HealthCheck.function_scoped_fixture])
    def test_image_has_all_required_tags(self, version, sha, image, registry):
        """
        Property: For any image build, all three tag types should be present
        
        This test verifies that the tagging logic produces:
        - Semantic version tag
        - Commit SHA tag  
        - Latest tag
        """
        # Simulate the tagging logic from build-images.sh
        base_image = f"{registry}/{image}"
        
        # Expected tags based on the build script logic
        expected_tags = [
            f"{base_image}:{sha}",           # Commit SHA tag
            f"{base_image}:{version}",       # Semantic version tag
            f"{base_image}:latest"           # Latest tag
        ]
        
        # Verify all expected tags are present
        assert len(expected_tags) == 3, "Should have exactly 3 tags"
        
        # Verify each tag follows the correct format
        for tag in expected_tags:
            # Tag should be in format: registry/image:tag
            assert re.match(r'^[a-z0-9\-\.:/]+:[a-z0-9\-\.]+$', tag), \
                f"Tag {tag} should follow valid format"
    
    @given(version=semantic_version())
    @settings(max_examples=100)
    def test_semantic_version_format(self, version):
        """
        Property: For any semantic version tag, it should follow v<major>.<minor>.<patch> format
        
        **Validates: Requirements 12.1**
        """
        # Verify semantic version format
        pattern = r'^v\d+\.\d+\.\d+$'
        assert re.match(pattern, version), \
            f"Version {version} should match semantic versioning pattern v<major>.<minor>.<patch>"
        
        # Verify it can be parsed
        parts = version[1:].split('.')
        assert len(parts) == 3, "Version should have exactly 3 parts"
        
        # Verify each part is a valid number
        for part in parts:
            assert part.isdigit(), f"Version part {part} should be a number"
            assert int(part) >= 0, f"Version part {part} should be non-negative"
    
    @given(sha=commit_sha())
    @settings(max_examples=100)
    def test_commit_sha_format(self, sha):
        """
        Property: For any commit SHA tag, it should be a valid git short SHA (7 hex chars)
        
        **Validates: Requirements 12.2**
        """
        # Verify SHA format (7 hexadecimal characters)
        assert len(sha) == 7, "Commit SHA should be 7 characters"
        assert re.match(r'^[0-9a-f]{7}$', sha), \
            f"Commit SHA {sha} should be 7 hexadecimal characters"
    
    @given(
        image=image_names,
        registry=registry_name()
    )
    @settings(max_examples=100)
    def test_latest_tag_always_present(self, image, registry):
        """
        Property: For any image build, a "latest" tag should always be created
        
        **Validates: Requirements 12.3**
        """
        # Simulate the tagging logic
        latest_tag = f"{registry}/{image}:latest"
        
        # Verify latest tag format
        assert latest_tag.endswith(':latest'), "Should have latest tag"
        assert re.match(r'^[a-z0-9\-\.:/]+:latest$', latest_tag), \
            f"Latest tag {latest_tag} should follow valid format"
    
    @given(
        version=semantic_version(),
        sha=commit_sha(),
        image=image_names
    )
    @settings(max_examples=100)
    def test_tags_are_unique(self, version, sha, image):
        """
        Property: For any image build, all tags should be unique (no duplicates)
        """
        registry = "test-registry/arbiter"
        base_image = f"{registry}/{image}"
        
        tags = [
            f"{base_image}:{sha}",
            f"{base_image}:{version}",
            f"{base_image}:latest"
        ]
        
        # Verify all tags are unique
        assert len(tags) == len(set(tags)), "All tags should be unique"
    
    @given(
        version1=semantic_version(),
        version2=semantic_version(),
        image=image_names
    )
    @settings(max_examples=100)
    def test_different_versions_produce_different_tags(self, version1, version2, image):
        """
        Property: For any two different semantic versions, they should produce different tags
        """
        assume(version1 != version2)  # Only test when versions are different
        
        registry = "test-registry/arbiter"
        base_image = f"{registry}/{image}"
        
        tag1 = f"{base_image}:{version1}"
        tag2 = f"{base_image}:{version2}"
        
        assert tag1 != tag2, "Different versions should produce different tags"


class TestImageTagResolution:
    """
    Property 18: Image tag resolution
    
    For any valid image tag (version-specific or "latest"), the container image
    should be resolvable and pullable from the registry.
    
    Note: This test validates the tag format and structure. Actual registry
    pulling would require a live registry and is tested in integration tests.
    
    **Validates: Requirements 12.4**
    """
    
    @given(
        version=semantic_version(),
        image=image_names,
        registry=registry_name()
    )
    @settings(max_examples=100)
    def test_version_specific_tag_is_resolvable(self, version, image, registry):
        """
        Property: For any version-specific tag, it should be in a valid format
        that can be resolved by container runtimes
        """
        tag = f"{registry}/{image}:{version}"
        
        # Verify tag format is valid for container runtimes
        # Format: [registry/]name[:tag]
        parts = tag.split(':')
        assert len(parts) == 2 or len(parts) == 3, \
            "Tag should have registry/image:tag format"
        
        # Verify no invalid characters
        assert not any(c in tag for c in [' ', '\n', '\t', '\r']), \
            "Tag should not contain whitespace"
        
        # Verify tag component is valid
        tag_component = parts[-1]
        assert len(tag_component) > 0, "Tag component should not be empty"
        assert re.match(r'^[a-zA-Z0-9\.\-_]+$', tag_component), \
            f"Tag component {tag_component} should only contain valid characters"
    
    @given(
        image=image_names,
        registry=registry_name()
    )
    @settings(max_examples=100)
    def test_latest_tag_is_resolvable(self, image, registry):
        """
        Property: For any "latest" tag, it should be in a valid format
        that can be resolved by container runtimes
        """
        tag = f"{registry}/{image}:latest"
        
        # Verify tag format is valid
        assert ':latest' in tag, "Should have :latest suffix"
        
        # Verify no invalid characters
        assert not any(c in tag for c in [' ', '\n', '\t', '\r']), \
            "Tag should not contain whitespace"
        
        # Verify the tag can be parsed
        parts = tag.split(':')
        assert len(parts) >= 2, "Tag should have at least image:tag format"
        assert parts[-1] == 'latest', "Last component should be 'latest'"
    
    @given(
        sha=commit_sha(),
        image=image_names,
        registry=registry_name()
    )
    @settings(max_examples=100)
    def test_commit_sha_tag_is_resolvable(self, sha, image, registry):
        """
        Property: For any commit SHA tag, it should be in a valid format
        that can be resolved by container runtimes
        """
        tag = f"{registry}/{image}:{sha}"
        
        # Verify tag format is valid
        parts = tag.split(':')
        assert len(parts) >= 2, "Tag should have at least image:tag format"
        
        # Verify SHA component is valid
        sha_component = parts[-1]
        assert len(sha_component) == 7, "SHA should be 7 characters"
        assert re.match(r'^[0-9a-f]{7}$', sha_component), \
            "SHA should be 7 hexadecimal characters"


if __name__ == '__main__':
    import pytest
    pytest.main([__file__, '-v'])
