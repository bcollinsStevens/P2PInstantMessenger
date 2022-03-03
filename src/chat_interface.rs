use crossterm::{
    event::{self, DisableMouseCapture, EnableMouseCapture, Event, KeyCode},
    execute,
    terminal::{disable_raw_mode, enable_raw_mode, EnterAlternateScreen, LeaveAlternateScreen},
};

use tui::{
    backend::CrosstermBackend,
    layout::{Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::{Span, Spans, Text},
    widgets::{Block, Borders, Paragraph},
    Terminal,
};

use std::{cmp::min, convert::TryFrom, error::Error, io, io::Stdout, u16::MAX as U16_MAX};

use unicode_width::UnicodeWidthStr;

enum InputMode {
    Normal,
    Editing,
}

pub struct ChatInterface {
    input: String,
    input_mode: InputMode,
    chat_history: Vec<String>,
    terminal: Terminal<CrosstermBackend<Stdout>>,
    quit_flag: bool,
    scroll_y: u16,
}

macro_rules! layout {
    () => {
        Layout::default()
            .direction(Direction::Vertical)
            .margin(2)
            .constraints(
                [
                    Constraint::Min(1),    // The Chat History
                    Constraint::Length(3), // The Input Field
                    Constraint::Length(1), // The Tooltip Text
                ]
                .as_ref(),
            )
    };
}

macro_rules! normal_tooltip {
    () => {
        Text::from(Spans::from(vec![
            Span::raw("Press "),
            Span::styled("q", Style::default().add_modifier(Modifier::BOLD)),
            Span::raw(" to exit, "),
            Span::styled("e", Style::default().add_modifier(Modifier::BOLD)),
            Span::raw(" to start editing, "),
            Span::styled("UP/DOWN", Style::default().add_modifier(Modifier::BOLD)),
            Span::raw(" to scroll."),
        ]))
    };
}

macro_rules! editing_tooltip {
    () => {
        Text::from(Spans::from(vec![
            Span::raw("Press "),
            Span::styled("Esc", Style::default().add_modifier(Modifier::BOLD)),
            Span::raw(" to stop editing, "),
            Span::styled("Enter", Style::default().add_modifier(Modifier::BOLD)),
            Span::raw(" to record the message"),
        ]))
    };
}

impl ChatInterface {
    pub fn new() -> Result<ChatInterface, Box<dyn Error>> {
        enable_raw_mode()?;
        let mut stdout = io::stdout();
        execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
        let backend = CrosstermBackend::new(stdout);
        let terminal = Terminal::new(backend)?;

        let interface = ChatInterface {
            input: String::new(),
            input_mode: InputMode::Normal,
            chat_history: Vec::new(),
            terminal: terminal,
            quit_flag: false,
            scroll_y: 0,
        };

        Ok(interface)
    }

    pub fn init(&self) -> Result<(), Box<dyn Error>> {
        enable_raw_mode()?;
        Ok(())
    }

    pub fn dinit(&mut self) -> Result<(), Box<dyn Error>> {
        disable_raw_mode()?;
        execute!(
            self.terminal.backend_mut(),
            LeaveAlternateScreen,
            DisableMouseCapture
        )?;
        self.terminal.show_cursor()?;
        Ok(())
    }

    pub fn do_input(&mut self) -> Result<(), Box<dyn Error>> {
        if let Event::Key(key) = event::read()? {
            match self.input_mode {
                InputMode::Normal => match key.code {
                    KeyCode::Char('e') => {
                        self.input_mode = InputMode::Editing;
                    }
                    KeyCode::Char('q') => {
                        self.quit_flag = true;
                    }
                    KeyCode::Up => {
                        if self.scroll_y > 0 {
                            self.scroll_y -= 1;
                        }
                    }
                    KeyCode::Down => {
                        if self.scroll_y < U16_MAX {
                            self.scroll_y += 1;
                        }
                    }
                    _ => {}
                },
                InputMode::Editing => match key.code {
                    KeyCode::Enter => {
                        if self.input.len() > 0 { // Dont send empty input
                            let input: String = self.input.drain(..).collect();
                            self.handle_input(input);
                        }
                    }
                    KeyCode::Char(c) => {
                        self.input.push(c);
                    }
                    KeyCode::Backspace => {
                        self.input.pop();
                    }
                    KeyCode::Esc => {
                        self.input_mode = InputMode::Normal;
                    }
                    _ => {}
                },
            };
        }
        Ok(())
    }

    pub fn check_quit(&self) -> bool {
        return self.quit_flag;
    }

    pub fn draw(&mut self) -> Result<(), Box<dyn Error>> {
        self.terminal.draw(|frame| {
            let rects = layout!().split(frame.size());
            let chat_history_rect = rects[0];
            let input_rect = rects[1];
            let tooltip_rect = rects[2];

            let tooltip_text = match self.input_mode {
                InputMode::Normal => normal_tooltip!(),
                InputMode::Editing => editing_tooltip!(),
            };

            let tooltip_widget = Paragraph::new(tooltip_text);
            frame.render_widget(tooltip_widget, tooltip_rect);

            let input_widget = Paragraph::new(self.input.as_ref())
                .style(match self.input_mode {
                    InputMode::Normal => Style::default(),
                    InputMode::Editing => Style::default().fg(Color::Yellow),
                })
                .block(Block::default().borders(Borders::ALL).title("Input"));
            frame.render_widget(input_widget, input_rect);

            match self.input_mode {
                InputMode::Normal => { // Hide the cursor. `Frame` does this by default, so we don't need to do anything here
                }
                InputMode::Editing => {
                    // Make the cursor visible and ask tui-rs to put it at the specified coordinates after rendering
                    frame.set_cursor(
                        input_rect.x + self.input.width() as u16 + 1, // Put cursor past the end of the input text
                        input_rect.y + 1, // Move one line down, from the border to the input line
                    )
                }
            }

            let mut message_spans: Vec<Spans> = self
                .chat_history
                .iter()
                .enumerate()
                .map(|(i, m)| {
                    Spans::from(vec![
                        Span::raw(format!("{}: ", i)),
                        Span::raw(m),
                        Span::raw("\n"),
                    ])
                })
                .collect();

            let num_messages = match u16::try_from(message_spans.len()).ok() {
                Some(v) => v,
                None => 0,
            };
            self.scroll_y = min(self.scroll_y, num_messages);
            if self.scroll_y > 0 && self.scroll_y < num_messages {
                message_spans.insert(
                    usize::from(self.scroll_y),
                    Spans::from(vec![Span::raw(format!(
                        "...{} messages above\n",
                        self.scroll_y
                    ))]),
                );
            } else if self.scroll_y >= num_messages {
                message_spans.push(Spans::from(vec![Span::raw(format!(
                    "...{} messages above\n",
                    self.scroll_y
                ))]));
            }

            let chat_history_widget = Paragraph::new(Text::from(message_spans))
                .block(Block::default().borders(Borders::ALL).title("Chat History"))
                .scroll((self.scroll_y, 0));
            frame.render_widget(chat_history_widget, chat_history_rect);
        })?;
        Ok(())
    }

    fn handle_input(&mut self, input: String) {
        self.chat_history.push(input);
    }

    pub fn push_message(&mut self, message: String) {
        self.chat_history.push(message);
    }
}
