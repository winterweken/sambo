//! A single-line text input.

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Input {
    value: String,
    /// Cursor position in characters, not bytes.
    cursor: usize,
    masked: bool,
}

impl Input {
    pub fn new(value: &str) -> Self {
        Input {
            value: value.to_string(),
            cursor: value.chars().count(),
            masked: false,
        }
    }

    /// An empty input whose value is hidden when displayed.
    pub fn masked() -> Self {
        Input {
            masked: true,
            ..Input::default()
        }
    }

    pub fn value(&self) -> &str {
        &self.value
    }

    pub fn cursor(&self) -> usize {
        self.cursor
    }

    pub fn is_masked(&self) -> bool {
        self.masked
    }

    fn len(&self) -> usize {
        self.value.chars().count()
    }

    fn byte_index(&self, chars: usize) -> usize {
        self.value
            .char_indices()
            .nth(chars)
            .map_or(self.value.len(), |(i, _)| i)
    }

    pub fn insert(&mut self, c: char) {
        self.value.insert(self.byte_index(self.cursor), c);
        self.cursor += 1;
    }

    pub fn backspace(&mut self) {
        if self.cursor > 0 {
            self.cursor -= 1;
            self.value.remove(self.byte_index(self.cursor));
        }
    }

    pub fn delete(&mut self) {
        if self.cursor < self.len() {
            self.value.remove(self.byte_index(self.cursor));
        }
    }

    pub fn left(&mut self) {
        self.cursor = self.cursor.saturating_sub(1);
    }

    pub fn right(&mut self) {
        self.cursor = (self.cursor + 1).min(self.len());
    }

    pub fn home(&mut self) {
        self.cursor = 0;
    }

    pub fn end(&mut self) {
        self.cursor = self.len();
    }

    /// The text to show: the value, or one bullet per character when masked.
    pub fn display(&self) -> String {
        if self.masked {
            "•".repeat(self.len())
        } else {
            self.value.clone()
        }
    }
}

#[cfg(test)]
mod tests;
