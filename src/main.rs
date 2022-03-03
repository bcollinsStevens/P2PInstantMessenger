mod chat_interface; // Tell rust to look for chat_interface.rs or chat_interface/mod.rs and import it under the name chat_interface
use chat_interface::ChatInterface; // Use ChatInterface from our module, chat_interface

use std::error::Error;

fn main() -> Result<(), Box<dyn Error>> {
    let mut interface = ChatInterface::new()?;
    interface.init()?;
    interface.push_message(String::from("Welcome to the Chat"));

    loop {
        interface.draw()?;
        interface.do_input()?;
        if interface.check_quit() {
            interface.dinit()?;
            break;
        }
    }

    Ok(())
}
