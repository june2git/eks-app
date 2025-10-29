package com.example.demo;

import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.ResponseBody;

@Controller
public class DemoController {

    @GetMapping("/index")
    @ResponseBody
    public String index() {
        return "Hello World";
    }

    @GetMapping("/health")
    @ResponseBody
    public String health() {
        return "OK";
    }

    @GetMapping("/test")
    @ResponseBody
    public String test() {
        return "test";
    }


    @GetMapping("/kafka")
    @ResponseBody
    public String kafka() {
        return "kafka!";
    }
}

