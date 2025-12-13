"""
Basic property-based test to verify Hypothesis configuration
"""

from hypothesis import given, strategies as st


@given(st.integers())
def test_hypothesis_setup(x: int):
    """Verify Hypothesis is configured correctly"""
    # Property: adding zero to any integer returns the same integer
    assert x + 0 == x


@given(st.text())
def test_hypothesis_with_strings(s: str):
    """Verify Hypothesis works with different data types"""
    # Property: concatenating empty string to any string returns the same string
    assert s + "" == s


if __name__ == "__main__":
    # Run tests directly
    test_hypothesis_setup()
    test_hypothesis_with_strings()
    print("✓ All property-based tests passed")
